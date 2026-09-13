// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestRedirectHandlerFlushesBeforePublishingCode(t *testing.T) {
	for _, body := range []string{"", defaultSuccessHTML, "<p>Signed in ✓</p>", strings.Repeat("✓", 4096)} {
		t.Run(strconv.Itoa(len(body)), func(t *testing.T) {
			ch := make(chan tokenResult, 1)
			w := &redirectResponseWriter{ResponseRecorder: httptest.NewRecorder()}
			w.onFlush = func() {
				if len(ch) != 0 {
					t.Error("authorization code published before flush")
				}
				resp := w.Result()
				defer func() { _ = resp.Body.Close() }()
				if resp.ContentLength != int64(len(body)) {
					t.Errorf("Content-Length = %d, want %d", resp.ContentLength, len(body))
				}
				if w.Body.String() != body {
					t.Error("complete response body not written before flush")
				}
			}
			h := redirectHandler{csrfToken: "csrf", ch: ch, doneCopy: body}
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/?state=csrf&code=code", nil))
			if !w.Flushed {
				t.Error("response not flushed before publishing authorization code")
			}
			select {
			case result := <-ch:
				if result.data != "code" || result.err != nil {
					t.Errorf("unexpected result: %+v", result)
				}
			default:
				t.Error("authorization code not published")
			}
		})
	}
}

func TestRedirectHandlerPublishesCodeOnResponseFailure(t *testing.T) {
	for _, failure := range []string{"write", "flush"} {
		t.Run(failure, func(t *testing.T) {
			ch := make(chan tokenResult, 1)
			w := &redirectResponseWriter{ResponseRecorder: httptest.NewRecorder()}
			if failure == "write" {
				w.writeErr = errors.New("browser disconnected during write")
			} else {
				w.flushErr = errors.New("browser disconnected during flush")
			}
			h := redirectHandler{csrfToken: "csrf", ch: ch, doneCopy: defaultSuccessHTML}
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/?state=csrf&code=code", nil))
			select {
			case result := <-ch:
				if result.data != "code" || result.err != nil {
					t.Errorf("response failure affected authorization code: %+v", result)
				}
			default:
				t.Error("authorization code not published")
			}
		})
	}
}

type redirectResponseWriter struct {
	*httptest.ResponseRecorder
	onFlush  func()
	writeErr error
	flushErr error
}

func (w *redirectResponseWriter) Write(body []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.ResponseRecorder.Write(body)
}

func (w *redirectResponseWriter) FlushError() error {
	if w.onFlush != nil {
		w.onFlush()
	}
	w.Flush()
	return w.flushErr
}

// TestNewClientWithPortsRedirectMatchesBind reproduces the hang seen on
// hosts where "localhost" resolves to ::1: the callback listener must be
// reachable at the literal host embedded in the redirect URL. The flow
// completes only if the redirect URL host and the bound socket agree on
// address family, which the IPv4-loopback bind guarantees.
func TestNewClientWithPortsRedirectMatchesBind(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "test-access-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		}))
	defer tokenSrv.Close()

	conf := oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "http://auth.example/authorize",
			TokenURL: tokenSrv.URL,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	visit := func(rawAuthURL string) error {
		u, err := url.Parse(rawAuthURL)
		if err != nil {
			return err
		}
		redirect := u.Query().Get("redirect_uri")
		ru, err := url.Parse(redirect)
		if err != nil {
			return err
		}
		if ru.Hostname() != "127.0.0.1" {
			t.Errorf("redirect host = %q, want 127.0.0.1", ru.Hostname())
		}
		q := ru.Query()
		q.Set("code", "test-auth-code")
		q.Set("state", u.Query().Get("state"))
		ru.RawQuery = q.Encode()

		go func() {
			resp, err := http.Get(ru.String())
			if err != nil {
				t.Errorf("redirect GET failed: %v", err)
				return
			}
			_ = resp.Body.Close()
		}()
		return nil
	}

	_, source, err := NewClientWithPorts(
		ctx, conf, visit, []int{0})
	if err != nil {
		t.Fatalf("NewClientWithPorts: %v", err)
	}
	tok, err := source.Token()
	if err != nil {
		t.Fatalf("source.Token: %v", err)
	}
	if tok.AccessToken != "test-access-token" {
		t.Fatalf("access token = %q, want test-access-token", tok.AccessToken)
	}
}

// TestServeRedirectsBindFailureNoPanic ensures a fully-exhausted port
// list reports the error instead of dereferencing a nil listener.
func TestServeRedirectsBindFailureNoPanic(t *testing.T) {
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = occupied.Close() }()
	port := occupied.Addr().(*net.TCPAddr).Port

	ready := make(chan readyResult, 1)
	var srv http.Server
	defer func() { _ = srv.Close() }()

	serveRedirects(context.Background(), &srv, "csrf",
		make(chan tokenResult), ready, "", []int{port})

	select {
	case res := <-ready:
		if res.err == nil {
			t.Fatalf("expected bind error, got addr %q", res.addr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveRedirects did not report bind failure")
	}
}
