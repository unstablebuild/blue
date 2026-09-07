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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	log "github.com/sirupsen/logrus"
)

const (
	authMiddlewareCallType = "AuthMiddleware"
)

// MiddlewareConfig configures the middleware returned by WithMiddleware.
type MiddlewareConfig[T any] struct {
	VerifyKeys   Keys
	Authorizer   Authorizer[T]
	SuccessLevel log.Level
	FailureLevel log.Level
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
	if config.SuccessLevel == 0 {
		config.SuccessLevel = log.InfoLevel
	}
	if config.FailureLevel == 0 {
		config.FailureLevel = log.WarnLevel
	}
	return &middleware[T]{
		verifyKeys:   config.VerifyKeys,
		authorizer:   config.Authorizer,
		next:         next,
		successLevel: config.SuccessLevel,
		failureLevel: config.FailureLevel,
	}
}

type middleware[T any] struct {
	verifyKeys   Keys
	authorizer   Authorizer[T]
	next         http.Handler
	successLevel log.Level
	failureLevel log.Level
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
		logging.LogResultLevel(m.successLevel, m.failureLevel,
			err, attemptAt, traceID, authMiddlewareCallType)
		return
	}

	if len(keys) == 0 {
		err := errors.New("no verify keys")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logging.LogResultLevel(m.successLevel, m.failureLevel,
			err, attemptAt, traceID, authMiddlewareCallType)
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
	logging.LogResultLevel(m.successLevel, m.failureLevel,
		nil, attemptAt, traceID, authMiddlewareCallType)
}

func (m *middleware[T]) forbidden(
	err error, w http.ResponseWriter, _ *http.Request, attemptAt time.Time,
	traceID trace.ID, fields ...logging.Field,
) {
	http.Error(w, err.Error(), http.StatusForbidden)
	logging.LogResultLevel(m.successLevel, m.failureLevel,
		err, attemptAt, traceID, authMiddlewareCallType, fields...)
}
