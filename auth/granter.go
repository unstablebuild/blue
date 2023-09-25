package auth

import "context"

// Granter abstracts the ability to grant claims to a user
// based on its userID and email.
type Granter[T any] interface {
	Grant(context.Context, *ProviderClaims) (T, error)
}

// FuncGranter uses fn to satisfy Granter.
func FuncGranter[T any](fn func(context.Context, *ProviderClaims) (T, error)) Granter[T] {
	return fnGranter[T]{fn: fn}
}

type fnGranter[T any] struct {
	fn func(context.Context, *ProviderClaims) (T, error)
}

func (f fnGranter[T]) Grant(ctx context.Context, claims *ProviderClaims) (T, error) {
	return f.fn(ctx, claims)
}
