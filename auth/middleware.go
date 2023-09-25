package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
	log "github.com/sirupsen/logrus"
)

const (
	authMiddlewareCallType = "AuthMiddleware"
)

// MiddlewareConfig configures the middleware returned by WithMiddleware.
type MiddlewareConfig[T any] struct {
	VerifyKeys Keys
	Authorizer Authorizer[T]
}

// WithMiddleware wraps next with a middleware that expects an oauth2 Authorization
// header to authenticate and extract user details to authorize a user.
// The Authorizer set in the config determines to what resources each user role
// has access to.
func WithMiddleware[T any](next http.Handler, config MiddlewareConfig[T]) http.Handler {
	if config.VerifyKeys == nil {
		panic("missing VerifyKeys in auth.MiddlewareConfig")
	}
	if config.Authorizer == nil {
		panic("missing Authorizer in auth.MiddlewareConfig")
	}
	return &middleware[T]{
		verifyKeys: config.VerifyKeys,
		authorizer: config.Authorizer,
		next:       next,
	}
}

type middleware[T any] struct {
	verifyKeys Keys
	authorizer Authorizer[T]
	client     http.Client
	next       http.Handler
}

func (m *middleware[T]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const bearerPrefix = "Bearer "

	traceID, ctx := trace.FromContextOrNew(r.Context())
	attemptAt := logAttempt(authMiddlewareCallType, r, traceID)

	bearerAuthToken := r.Header.Get("Authorization")

	if !strings.HasPrefix(bearerAuthToken, bearerPrefix) {
		err := fmt.Errorf("validate token request: bearer not found: %q",
			bearerAuthToken)
		m.forbidden(err, w, r, attemptAt, traceID)
		return
	}

	authToken := bearerAuthToken[len(bearerPrefix):]

	keys, err := m.verifyKeys.Verify(ctx)
	if err != nil {
		err := fmt.Errorf("get verify key: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logging.LogResultInfo(err, attemptAt, traceID, authMiddlewareCallType)
		return
	}

	if len(keys) == 0 {
		err := errors.New("no verify keys")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logging.LogResultInfo(err, attemptAt, traceID, authMiddlewareCallType)
		return
	}

	var claims UserClaims[T]
	for _, key := range keys {
		claims, err = VerifyToken[T](key, authToken)
		if err == nil {
			break
		}
	}
	if err != nil {
		m.forbidden(err, w, r, attemptAt, traceID)
		return
	}

	if err := m.authorizer.Authorize(ctx, claims, r.RequestURI); err != nil {
		m.forbidden(err, w, r, attemptAt, traceID)
		return
	}

	r = r.WithContext(ContextWithClaims(ctx, claims))
	m.next.ServeHTTP(w, r)
	logging.LogResult(nil, attemptAt, traceID, authMiddlewareCallType)
}

func (m *middleware[T]) forbidden(
	err error, w http.ResponseWriter, r *http.Request, attemptAt time.Time,
	traceID trace.ID, fields ...logging.Field,
) {
	http.Error(w, err.Error(), http.StatusForbidden)
	logging.LogResultLevel(log.DebugLevel, log.WarnLevel, err, attemptAt, traceID, authMiddlewareCallType, fields...)
}
