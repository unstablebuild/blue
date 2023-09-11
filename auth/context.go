package auth

import (
	"context"
)

type ctxKey int

var claimsKey ctxKey

// ContextWithClaims returns a new Context that holds claims.
func ContextWithClaims[T any](ctx context.Context, claims UserClaims[T]) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// ClaimsFromContext returns the claims value stored in ctx, if any.
func ClaimsFromContext[T any](ctx context.Context) (UserClaims[T], bool) {
	claims, ok := ctx.Value(claimsKey).(UserClaims[T])
	return claims, ok
}
