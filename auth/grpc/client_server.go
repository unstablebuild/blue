package grpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ernestrc/blue/auth"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
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
	signKey []byte,
	authorizer auth.Authorizer[T],
	creds credentials.TransportCredentials,
) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.UnaryInterceptor(oauth2UnaryInterceptor(signKey, authorizer)),
		grpc.Creds(creds),
	}
}

func oauth2UnaryInterceptor[T any](
	signKey []byte, authorizer auth.Authorizer[T],
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		const (
			callType     = "auth.UnaryInterceptor"
			bearerPrefix = "Bearer "
		)

		traceID, ctx := trace.FromContextOrNew(ctx)
		fields := []logging.Field{
			{Key: logging.KeyClass, Value: "auth.grpc"},
			{Key: "Method", Value: info.FullMethod},
		}
		attemptAt := logging.LogAttempt(traceID, callType, fields...)

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			err := status.Errorf(codes.InvalidArgument, "missing metadata")
			logging.LogResult(err, attemptAt, traceID, callType, fields...)
			return nil, err
		}

		// The keys within metadata.MD are normalized to lowercase.
		// See: https://godoc.org/google.golang.org/grpc/metadata#New
		bearerAuthTokenMulti := md["authorization"]
		if len(bearerAuthTokenMulti) == 0 || bearerAuthTokenMulti[0] == "" {
			err := status.Errorf(codes.InvalidArgument, "missing authorization in metadata")
			logging.LogResult(err, attemptAt, traceID, callType, fields...)
			return nil, err
		}

		bearerAuthToken := bearerAuthTokenMulti[0]

		if !strings.HasPrefix(bearerAuthToken, bearerPrefix) {
			msg := fmt.Sprintf("validate token request: bearer not found: %q",
				bearerAuthToken)
			err := status.Errorf(codes.PermissionDenied, msg)
			logging.LogResult(err, attemptAt, traceID, callType, fields...)
			return nil, err
		}

		authToken := bearerAuthToken[len(bearerPrefix):]
		claims, err := auth.VerifyToken[T](signKey, authToken)
		if err != nil {
			err := status.Errorf(codes.PermissionDenied, err.Error())
			logging.LogResult(err, attemptAt, traceID, callType, fields...)
			return nil, err
		}

		if err := authorizer.Authorize(ctx, claims, info.FullMethod); err != nil {
			err := status.Errorf(codes.PermissionDenied, err.Error())
			logging.LogResult(err, attemptAt, traceID, callType, fields...)
			return nil, err
		}

		logging.LogResult(nil, attemptAt, traceID, callType, fields...)
		return handler(ctx, req)
	}
}
