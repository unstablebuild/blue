package auth

import "context"

// Granter abstracts the ability to grant claims to a user
// based on its userID and email.
type Granter[T any] interface {
	Grant(ctx context.Context, userID, email string) (T, error)
}

// FuncGranter uses fn to satisfy Granter.
func FuncGranter[T any](fn func(context.Context, string, string) (T, error)) Granter[T] {
	return fnGranter[T]{fn: fn}
}

type fnGranter[T any] struct {
	fn func(context.Context, string, string) (T, error)
}

func (f fnGranter[T]) Grant(ctx context.Context, userID, email string) (T, error) {
	return f.fn(ctx, userID, email)
}
