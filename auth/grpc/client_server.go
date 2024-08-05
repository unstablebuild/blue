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

package grpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GrpcWithOauth2 returns a grpc.DialOption to be used in calls to grpc.Dial,
func GRPCClientWithOauth2(
	source oauth2.TokenSource, creds credentials.TransportCredentials,
) []grpc.DialOption {
	perRPC := oauth.TokenSource{TokenSource: source}
	return []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(perRPC),
	}
}

// GRPCServerWithOauth2 returns a set of grpc.ServerOption that configure the server
// to allow oauth2 requests only.
func GRPCServerWithOauth2[T any](
	verifyKeys auth.Keys,
	authorizer auth.Authorizer[T],
	creds credentials.TransportCredentials,
) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.UnaryInterceptor(oauth2UnaryInterceptor[T](verifyKeys, authorizer)),
		grpc.StreamInterceptor(oauth2StreamInterceptor[T](verifyKeys, authorizer)),
		grpc.Creds(creds),
	}
}

// GRPCServerWithInsecureOauth2 returns a set of grpc.ServerOption that configure the server
// to allow oauth2 requests only, without TLS credentials. This should only be used
// when an alternate method of securing the transport is used (i.e. TCP/TLS load balancer, etc.),
// or for debugging or local testing.
func GRPCServerWithInsecureOauth2[T any](
	verifyKeys auth.Keys,
	authorizer auth.Authorizer[T],
) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.UnaryInterceptor(oauth2UnaryInterceptor(verifyKeys, authorizer)),
		grpc.StreamInterceptor(oauth2StreamInterceptor(verifyKeys, authorizer)),
	}
}

func oauth2StreamInterceptor[T any](
	verifyKeys auth.Keys, authorizer auth.Authorizer[T],
) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream,
		info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		claims, err := authenticate(ss.Context(), "auth.StreamInterceptor",
			info.FullMethod, verifyKeys, authorizer)
		if err != nil {
			return err
		}
		ctx := auth.ContextWithClaims(ss.Context(), claims)
		ss = contextServerStream{ctx: ctx, ss: ss}
		return handler(srv, ss)
	}
}

func oauth2UnaryInterceptor[T any](
	verifyKeys auth.Keys, authorizer auth.Authorizer[T],
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		claims, err := authenticate(ctx, "auth.UnaryInterceptor",
			info.FullMethod, verifyKeys, authorizer)
		if err != nil {
			return nil, err
		}
		ctx = auth.ContextWithClaims(ctx, claims)
		return handler(ctx, req)
	}
}

func authenticate[T any](
	ctx context.Context, callType, method string,
	verifyKeys auth.Keys, authorizer auth.Authorizer[T],
) (auth.UserClaims[T], error) {
	const bearerPrefix = "Bearer "

	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "auth.grpc"},
		{Key: "Method", Value: method},
	}
	attemptAt := logging.LogAttempt(traceID, callType, fields...)

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		err := status.Errorf(codes.InvalidArgument, "missing metadata")
		logging.LogResult(err, attemptAt, traceID, callType, fields...)
		return auth.UserClaims[T]{}, err
	}

	// The keys within metadata.MD are normalized to lowercase.
	// See: https://godoc.org/google.golang.org/grpc/metadata#New
	bearerAuthTokenMulti := md["authorization"]
	if len(bearerAuthTokenMulti) == 0 || bearerAuthTokenMulti[0] == "" {
		err := status.Errorf(codes.InvalidArgument, "missing authorization in metadata")
		logging.LogResult(err, attemptAt, traceID, callType, fields...)
		return auth.UserClaims[T]{}, err
	}

	bearerAuthToken := bearerAuthTokenMulti[0]

	if !strings.HasPrefix(bearerAuthToken, bearerPrefix) {
		msg := fmt.Sprintf("validate token request: bearer not found: %q",
			bearerAuthToken)
		err := status.Errorf(codes.PermissionDenied, msg)
		logging.LogResult(err, attemptAt, traceID, callType, fields...)
		return auth.UserClaims[T]{}, err
	}

	keys, err := verifyKeys.Verify(ctx)
	if err != nil {
		err = fmt.Errorf("get verify keys: %v", err)
		err := status.Errorf(codes.PermissionDenied, err.Error())
		logging.LogResult(err, attemptAt, traceID, callType, fields...)
		return auth.UserClaims[T]{}, err
	}

	authToken := bearerAuthToken[len(bearerPrefix):]
	var claims auth.UserClaims[T]
	for _, k := range keys {
		claims, err = auth.VerifyToken[T](k, authToken)
		if err == nil {
			break
		}
	}
	if err != nil {
		err := status.Errorf(codes.PermissionDenied, err.Error())
		logging.LogResult(err, attemptAt, traceID, callType, fields...)
		return auth.UserClaims[T]{}, err
	}

	if err := authorizer.Authorize(ctx, claims, method); err != nil {
		err := status.Errorf(codes.PermissionDenied, err.Error())
		logging.LogResult(err, attemptAt, traceID, callType, fields...)
		return auth.UserClaims[T]{}, err
	}

	logging.LogResult(nil, attemptAt, traceID, callType, fields...)
	return claims, nil
}
