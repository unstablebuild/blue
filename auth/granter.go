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
