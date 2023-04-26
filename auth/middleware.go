package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
)

const (
	authMiddlewareCallType = "AuthMiddleware"
)

// MiddlewareConfig configures the middleware returned by WithMiddleware.
type MiddlewareConfig[T any] struct {
	SignKey    []byte
	Authorizer Authorizer[T]
}

// WithMiddleware wraps next with a middleware that expects an oauth2 Authorization
// header to authenticate and extract user details to authorize a user.
// The Authorizer set in the config determines to what resources each user role
// has access to.
func WithMiddleware[T any](next http.Handler, config MiddlewareConfig[T]) http.Handler {
	return &middleware[T]{
		signKey:    config.SignKey,
		authorizer: config.Authorizer,
		next:       next,
	}
}

type middleware[T any] struct {
	signKey    []byte
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
	claims, err := VerifyToken[T](m.signKey, authToken)
	if err != nil {
		m.forbidden(err, w, r, attemptAt, traceID)
		return
	}

	if err := m.authorizer.Authorize(ctx, claims, r.RequestURI); err != nil {
		m.forbidden(err, w, r, attemptAt, traceID)
		return
	}

	m.next.ServeHTTP(w, r)
}

func (m *middleware[T]) forbidden(
	err error, w http.ResponseWriter, r *http.Request, attemptAt time.Time,
	traceID trace.ID, fields ...logging.Field,
) {
	http.Error(w, err.Error(), http.StatusForbidden)
	logging.LogResultInfo(err, attemptAt, traceID, authMiddlewareCallType, fields...)
}
