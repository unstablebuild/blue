// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
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
