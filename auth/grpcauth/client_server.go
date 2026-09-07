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

package grpcauth

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
	return GRPCServerWithOauth2Interceptors(
		Oauth2UnaryInterceptor[T](verifyKeys, authorizer),
		Oauth2StreamInterceptor[T](verifyKeys, authorizer),
		creds,
	)
}

// GRPCServerWithInsecureOauth2 returns a set of grpc.ServerOption that configure the server
// to allow oauth2 requests only, without TLS credentials. This should only be used
// when an alternate method of securing the transport is used (i.e. TCP/TLS load balancer, etc.),
// or for debugging or local testing.
func GRPCServerWithInsecureOauth2[T any](
	verifyKeys auth.Keys,
	authorizer auth.Authorizer[T],
) []grpc.ServerOption {
	return GRPCServerWithInsecureOauth2Interceptors(
		Oauth2UnaryInterceptor[T](verifyKeys, authorizer),
		Oauth2StreamInterceptor[T](verifyKeys, authorizer),
	)
}

// GRPCServerWithOauth2Interceptors returns the server options for an oauth2
// server using the given interceptors. Callers that need to wrap the oauth2
// interceptors (e.g. to cache verification/authorization) pass the wrapped
// interceptors here instead of using GRPCServerWithOauth2.
func GRPCServerWithOauth2Interceptors(
	unary grpc.UnaryServerInterceptor,
	stream grpc.StreamServerInterceptor,
	creds credentials.TransportCredentials,
) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.UnaryInterceptor(unary),
		grpc.StreamInterceptor(stream),
		grpc.Creds(creds),
	}
}

// GRPCServerWithInsecureOauth2Interceptors is like
// GRPCServerWithOauth2Interceptors but without TLS credentials. This should
// only be used when the transport is secured externally (TCP/TLS load
// balancer, etc.), or for debugging or local testing.
func GRPCServerWithInsecureOauth2Interceptors(
	unary grpc.UnaryServerInterceptor,
	stream grpc.StreamServerInterceptor,
) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.UnaryInterceptor(unary),
		grpc.StreamInterceptor(stream),
	}
}

// Oauth2StreamInterceptor returns the stream interceptor that authenticates
// and authorizes oauth2 requests and installs the resulting claims in the
// stream context. It is exposed so callers can wrap it (e.g. to add a cache
// in front of the per-RPC verification/authorization).
func Oauth2StreamInterceptor[T any](
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

// Oauth2UnaryInterceptor returns the unary interceptor that authenticates
// and authorizes oauth2 requests and installs the resulting claims in the
// request context. It is exposed so callers can wrap it (e.g. to add a cache
// in front of the per-RPC verification/authorization).
func Oauth2UnaryInterceptor[T any](
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
		err := status.Errorf(codes.PermissionDenied, "%s", msg)
		logging.LogResult(err, attemptAt, traceID, callType, fields...)
		return auth.UserClaims[T]{}, err
	}

	keys, err := verifyKeys.Verify(ctx)
	if err != nil {
		err = fmt.Errorf("get verify keys: %v", err)
		err := status.Errorf(codes.PermissionDenied, "%s", err.Error())
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
		err := status.Errorf(codes.PermissionDenied, "%s", err.Error())
		logging.LogResult(err, attemptAt, traceID, callType, fields...)
		return auth.UserClaims[T]{}, err
	}

	if err := authorizer.Authorize(ctx, claims, method); err != nil {
		err := status.Errorf(codes.PermissionDenied, "%s", err.Error())
		logging.LogResult(err, attemptAt, traceID, callType, fields...)
		return auth.UserClaims[T]{}, err
	}

	logging.LogResult(nil, attemptAt, traceID, callType, fields...)
	return claims, nil
}
