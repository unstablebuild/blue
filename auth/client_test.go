// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.


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
