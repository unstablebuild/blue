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
	"errors"
)

var (
	// ErrForbidden is returned by Authorizer when a user's privileges do not allow
	// access to the underlying resource.
	ErrForbidden = errors.New("user is not authorized to access resource")
)

// Authorizer abstract the ability to authorize a user for a resource.
type Authorizer[T any] interface {
	Authorize(ctx context.Context, user UserClaims[T], resource string) error
}

// AuthorizeAll returns an Authorizer that authorizes access to all resources
// to any user.
func AuthorizeAll[T any]() Authorizer[T] {
	return authorizeAll[T]{}
}

// FuncAuthorizer returns an authorizer that calls fn to Authorize a user.
func FuncAuthorizer[T any](
	fn func(context.Context, UserClaims[T], string) error,
) Authorizer[T] {
	return funcAuthorize[T](fn)
}

type funcAuthorize[T any] func(context.Context, UserClaims[T], string) error

type authorizeAll[T any] struct {
}

func (g authorizeAll[T]) Authorize(
	ctx context.Context, user UserClaims[T], resource string,
) error {
	return nil
}

func (g funcAuthorize[T]) Authorize(
	ctx context.Context, user UserClaims[T], resource string,
) error {
	return g(ctx, user, resource)
}
