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
)

type ctxKey int

const (
	claimsKey ctxKey = iota
	clientIDKey
)

// ContextWithClaims returns a new Context that holds claims.
func ContextWithClaims[T any](ctx context.Context, claims UserClaims[T]) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// ClaimsFromContext returns the claims value stored in ctx, if any.
func ClaimsFromContext[T any](ctx context.Context) (UserClaims[T], bool) {
	claims, ok := ctx.Value(claimsKey).(UserClaims[T])
	return claims, ok
}

// ClientIDFromContext returns the OAuth2 client a Granter is granting
// for. TokenHTTPHandler sets it only after the provider's ID token was
// validated with that client as its audience, so it names the client the
// provider issued the tokens to, not merely the one the request claimed.
// It is absent outside of a Granter called by TokenHTTPHandler.
func ClientIDFromContext(ctx context.Context) (string, bool) {
	clientID, ok := ctx.Value(clientIDKey).(string)
	return clientID, ok
}

// ContextWithClientID returns a new Context that holds clientID for
// ClientIDFromContext, as TokenHTTPHandler hands it to a Granter.
func ContextWithClientID(ctx context.Context, clientID string) context.Context {
	return context.WithValue(ctx, clientIDKey, clientID)
}
