package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/authhandler"
)

const (
	redirectCallType = "oauth2Redirect"
	loggingClass     = "auth"
)

// NewClient starts a oauth2 flow with the given oaut2 config
// and returns an *http.Client that will refresh the token as necessary,
// or an error if there's an error completing the oauth2 flow.
//
// This client can be used against WithMiddleware if configured
// to use a Token endpoint managed by the handler returned by
// TokenHTTPHandler.
func NewClient(
	ctx context.Context, conf oauth2.Config,
	visitURLCallback func(string) error, successBrowserCopy string,
) (*http.Client, oauth2.TokenSource, error) {
	const oauth2FlowTimeout = 60 * time.Second
	ctx, cancel := context.WithTimeout(ctx, oauth2FlowTimeout)
	defer cancel()

	resChan := make(chan tokenResult)
	readyChan := make(chan readyResult)
	csrfToken := uuid.New().String()
	usePKCE := conf.ClientSecret == ""

	var srv http.Server
	defer srv.Close()

	go serveRedirects(ctx, &srv, csrfToken, resChan,
		readyChan, successBrowserCopy)

	readyResult := <-readyChan
	if readyResult.err != nil {
		return nil, nil, fmt.Errorf("serve: %v", readyResult.err)
	}

	conf.RedirectURL = fmt.Sprintf("http://localhost:%d/o/oauth2/redirect",
		readyResult.port)

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
	url := conf.AuthCodeURL(csrfToken, authCodeURLOpts...)
	if err := visitURLCallback(url); err != nil {
		return nil, nil, err
	}

	// Use the authorization code that is pushed to the redirect
	// URL. Exchange will do the handshake to retrieve the
	// initial access token.
	result := <-resChan

	if usePKCE {
		pkceOpts = append(pkceOpts, oauth2.SetAuthURLParam("verifier", pkce.Verifier))
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
) {
	handler := redirectHandler{doneCopy: doneCopy, csrfToken: csrfToken, ch: done}
	srv.Handler = handler

	ln, err := net.Listen("tcp", srv.Addr)
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
	ctx context.Context, callType string, traceID trace.ID,
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
		logging.LogResultInfo(err, attemptAt, traceID, redirectCallType, fields...)
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
	h.ch <- tokenResult{data: code}

	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "redirectHandler"},
		{Key: "Method", Value: r.Method},
		{Key: "URL", Value: r.URL.String()},
	}

	w.Header().Set("Content-Type", "text/html")

	_, err := w.Write([]byte(h.doneCopy))
	logging.LogResultInfo(err, attemptAt, traceID, redirectCallType, fields...)
}
