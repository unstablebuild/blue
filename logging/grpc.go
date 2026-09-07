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

package logging

import (
	"context"

	"github.com/unstablebuild/blue/logging/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const metadataTraceID = "X-Blue-TraceID"

func appendTraceToOutgoingContext(ctx context.Context, traceID trace.ID) context.Context {
	return metadata.AppendToOutgoingContext(
		ctx, metadataTraceID, string(traceID))
}

func traceFromMetadata(ctx context.Context) (context.Context, trace.ID) {
	var traceID trace.ID

	md, ok := metadata.FromIncomingContext(ctx)
	if ok && len(md.Get(metadataTraceID)) != 0 {
		traceID = trace.ID(md.Get(metadataTraceID)[0])
	} else {
		traceID = trace.New()
		ctx = appendTraceToOutgoingContext(ctx, traceID)
	}

	return ctx, traceID
}

func clientInterceptor(
	ctx context.Context,
	method string,
	req interface{},
	reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {

	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := LogAttempt(traceID, method)

	ctx = appendTraceToOutgoingContext(ctx, traceID)
	err := invoker(ctx, method, req, reply, cc, opts...)

	LogResult(err, attemptAt, traceID, method)

	return err
}

func clientStreamingInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {

	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := LogAttempt(traceID, method)

	ctx = appendTraceToOutgoingContext(ctx, traceID)
	cs, err := streamer(ctx, desc, cc, method, opts...)

	LogResult(err, attemptAt, traceID, method)

	return cs, err
}

func serverInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {

	var traceID trace.ID
	ctx, traceID = traceFromMetadata(ctx)
	attemptAt := LogAttempt(traceID, info.FullMethod)

	ctx = trace.NewContext(ctx, traceID)
	h, err := handler(ctx, req)

	LogResult(err, attemptAt, traceID, info.FullMethod)

	return h, err
}

func serverStreamingInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	_, traceID := traceFromMetadata(ss.Context())
	attemptAt := LogAttempt(traceID, info.FullMethod)

	// NOTE: no traceID is passed downstream.o
	// grpc.ServerStream doesn't doesn't allow for an easy
	// way to intercept calls to Context()
	//
	// ctx = trace.NewContext(ctx, traceID)
	err := handler(srv, ss)

	LogResult(err, attemptAt, traceID, info.FullMethod)

	return err
}

// WithClientLogger returns a UnaryInterceptor and a StreamInterceptor options
// which log GRPC client invocations and results.
func WithClientLogger() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(clientInterceptor),
		grpc.WithStreamInterceptor(clientStreamingInterceptor),
	}
}

// WithServerLogger returns a UnaryInterceptor and a StreamInterceptor options
// which log GRPC server invocations and results.
func WithServerLogger() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.UnaryInterceptor(serverInterceptor),
		grpc.StreamInterceptor(serverStreamingInterceptor),
	}
}
