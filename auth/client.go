// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
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
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/authhandler"
)

const (
	redirectCallType   = "oauth2Redirect"
	loggingClass       = "auth"
	defaultRedirectPath = "/o/oauth2/redirect"
)

// ClientOption configures the OAuth2 client flow.
type ClientOption func(*clientConfig)

type clientConfig struct {
	redirectPath    string
	authCodeOptions []oauth2.AuthCodeOption
}

// WithRedirectPath overrides the default callback path
// ("/o/oauth2/redirect") used in the redirect URL. Use this when
// the OAuth provider has a specific redirect URI registered
// (e.g. "/auth/callback").
func WithRedirectPath(path string) ClientOption {
	return func(c *clientConfig) { c.redirectPath = path }
}

// WithAuthCodeOptions appends extra parameters to the authorization
// URL (e.g. oauth2.SetAuthURLParam("prompt", "consent")).
func WithAuthCodeOptions(opts ...oauth2.AuthCodeOption) ClientOption {
	return func(c *clientConfig) { c.authCodeOptions = append(c.authCodeOptions, opts...) }
}

// NewClient starts an oauth2 like NewClientWithPorts but
// will setup a callback listener on a random port.
// See NewClientWithPorts for more details.
func NewClient(
	ctx context.Context, conf oauth2.Config,
	visitURLCallback func(string) error, successBrowserCopy string,
	opts ...ClientOption,
) (*http.Client, oauth2.TokenSource, error) {
	return NewClientWithPorts(ctx, conf, visitURLCallback, successBrowserCopy, nil, opts...)
}

// NewClientWithPorts starts a oauth2 flow with the given oaut2 config
// and returns an *http.Client that will refresh the token as necessary,
// or an error if there's an error completing the oauth2 flow.
//
// This client can be used against WithMiddleware if configured
// to use a Token endpoint managed by the handler returned by
// TokenHTTPHandler.
//
// Oftentimes oauth2 providers want the callback url to be in
// a whitelist so listening on a random port won't work. The
// parameter tryPorts is designed to enable users to whitelist
// a (hopefully long) list of known ports and pass them
// to this constructor.
func NewClientWithPorts(
	ctx context.Context, conf oauth2.Config,
	visitURLCallback func(string) error, successBrowserCopy string,
	tryPorts []int, opts ...ClientOption,
) (*http.Client, oauth2.TokenSource, error) {
	cfg := clientConfig{redirectPath: defaultRedirectPath}
	for _, o := range opts {
		o(&cfg)
	}

	resChan := make(chan tokenResult)
	readyChan := make(chan readyResult)
	csrfToken := uuid.New().String()
	usePKCE := conf.ClientSecret == ""

	var srv http.Server
	defer func() { _ = srv.Close() }()

	go serveRedirects(ctx, &srv, csrfToken, resChan,
		readyChan, successBrowserCopy, tryPorts)

	select {
	case readyResult := <-readyChan:
		if readyResult.err != nil {
			return nil, nil, fmt.Errorf("serve: %v", readyResult.err)
		}
		conf.RedirectURL = fmt.Sprintf("http://localhost:%d%s",
			readyResult.port, cfg.redirectPath)
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	var pkceOpts []oauth2.AuthCodeOption
	var pkce authhandler.PKCEParams
	var err error
	if usePKCE {
		pkce, err = generatePKCEParams(defaultLength)
		if err != nil {
			return nil, nil, fmt.Errorf("generate pkce params: %v", err)
		}
		pkceOpts = append(pkceOpts, oauth2.SetAuthURLParam("code_challenge", pkce.Challenge))
		pkceOpts = append(pkceOpts, oauth2.SetAuthURLParam("code_challenge_method", pkce.ChallengeMethod))
	}
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	authCodeURLOpts := append([]oauth2.AuthCodeOption{oauth2.AccessTypeOffline}, pkceOpts...)
	authCodeURLOpts = append(authCodeURLOpts, cfg.authCodeOptions...)
	url := conf.AuthCodeURL(csrfToken, authCodeURLOpts...)
	if err := visitURLCallback(url); err != nil {
		return nil, nil, err
	}

	// Use the authorization code that is pushed to the redirect
	// URL. Exchange will do the handshake to retrieve the
	// initial access token.
	var result tokenResult
	select {
	case result = <-resChan:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	if usePKCE {
		pkceOpts = append(pkceOpts, oauth2.SetAuthURLParam("code_verifier", pkce.Verifier))
	}
	tok, err := conf.Exchange(ctx, result.data, pkceOpts...)
	if err != nil {
		return nil, nil, err
	}

	refreshCtx := context.Background()
	source := conf.TokenSource(refreshCtx, tok)
	return oauth2.NewClient(refreshCtx, source), source, nil
}

type response struct {
	Message string
	Success bool
	Data    string
}

type tokenResult struct {
	err  error
	data string
}

type readyResult struct {
	err  error
	port int
}

func serveRedirects(
	ctx context.Context, srv *http.Server, csrfToken string,
	done chan tokenResult, ready chan readyResult, doneCopy string,
	knownPorts []int,
) {
	handler := redirectHandler{doneCopy: doneCopy, csrfToken: csrfToken, ch: done}
	srv.Handler = handler

	var err error
	var ln net.Listener
	if len(knownPorts) == 0 {
		ln, err = net.Listen("tcp", srv.Addr)
	} else {
		for _, port := range knownPorts {
			ln, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err == nil {
				break
			}
		}
	}
	if err != nil {
		select {
		case ready <- readyResult{err: err}:
		case <-ctx.Done():
		}
	}

	ready <- readyResult{port: ln.Addr().(*net.TCPAddr).Port}
	if err := srv.Serve(ln); err != nil {
		select {
		case done <- tokenResult{err: err}:
		case <-ctx.Done():
		}
	}
}

func logAttempt(callType string, req *http.Request, traceID trace.ID) time.Time {
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: loggingClass},
		{Key: "Method", Value: req.Method},
		{Key: "URL", Value: req.URL.String()},
	}
	return logging.LogAttempt(traceID, callType, fields...)
}

func writeResponse(
	_ context.Context, callType string, traceID trace.ID,
	attemptAt time.Time, w http.ResponseWriter,
	req *http.Request, status int, r response,
) {
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: loggingClass},
		{Key: "Method", Value: req.Method},
		{Key: "URL", Value: req.URL.String()},
		{Key: "Status", Value: strconv.Itoa(status)},
		{Key: "ResponseMessage", Value: r.Message},
		{Key: "ResponseSuccess", Value: strconv.FormatBool(r.Success)},
	}
	data, err := json.Marshal(r)
	if err != nil {
		logging.LogResultInfo(err, attemptAt, traceID, callType, fields...)
		return
	}

	w.WriteHeader(status)
	_, err = w.Write(data)
	logging.LogResultInfo(err, attemptAt, traceID, callType, fields...)
}

type redirectHandler struct {
	csrfToken string
	ch        chan tokenResult
	doneCopy  string
}

func (h redirectHandler) ServeHTTP(
	w http.ResponseWriter, r *http.Request,
) {
	traceID, ctx := trace.FromContextOrNew(r.Context())
	attemptAt := logAttempt(redirectCallType, r, traceID)

	queryparams := r.URL.Query()

	actualCsrfToken := queryparams.Get("state")
	if actualCsrfToken != h.csrfToken {
		writeResponse(ctx, redirectCallType, traceID, attemptAt,
			w, r, http.StatusBadRequest, response{Message: "invalid redirect state"})
		return
	}

	code := queryparams.Get("code")
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "redirectHandler"},
		{Key: "Method", Value: r.Method},
		{Key: "URL", Value: r.URL.String()},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	body := h.doneCopy
	if body == "" {
		body = defaultSuccessHTML
	}
	_, err := w.Write([]byte(body))

	h.ch <- tokenResult{data: code}
	logging.LogResultInfo(err, attemptAt, traceID, redirectCallType, fields...)
}
