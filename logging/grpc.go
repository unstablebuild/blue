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
