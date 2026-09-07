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
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

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
