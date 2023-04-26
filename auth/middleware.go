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
type MiddlewareConfig struct {
	SignKey    []byte
	Authorizer Authorizer[UserClaims]
}

// WithMiddleware wraps next with a middleware that expects an oauth2 Authorization
// header to authenticate and extract user details to authorize a user.
// The Authorizer set in the config determines to what resources each user role
// has access to.
func WithMiddleware(next http.Handler, config MiddlewareConfig) http.Handler {
	return &middleware{
		signKey:    config.SignKey,
		authorizer: config.Authorizer,
		next:       next,
	}
}

type middleware struct {
	signKey    []byte
	authorizer Authorizer[UserClaims]
	client     http.Client
	next       http.Handler
}

func (m *middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	claims, err := VerifyToken(m.signKey, authToken)
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

func (m *middleware) forbidden(
	err error, w http.ResponseWriter, r *http.Request, attemptAt time.Time,
	traceID trace.ID, fields ...logging.Field,
) {
	http.Error(w, err.Error(), http.StatusForbidden)
	logging.LogResultInfo(err, attemptAt, traceID, authMiddlewareCallType, fields...)
}
