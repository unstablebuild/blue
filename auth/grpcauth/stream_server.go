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

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type contextServerStream struct {
	ctx context.Context
	ss  grpc.ServerStream
}

func (c contextServerStream) SetHeader(md metadata.MD) error {
	return c.ss.SetHeader(md)
}

func (c contextServerStream) SendHeader(md metadata.MD) error {
	return c.ss.SendHeader(md)
}

func (c contextServerStream) SetTrailer(md metadata.MD) {
	c.ss.SetTrailer(md)
}

func (c contextServerStream) Context() context.Context {
	return c.ctx
}

func (c contextServerStream) SendMsg(m interface{}) error {
	return c.ss.SendMsg(m)
}

func (c contextServerStream) RecvMsg(m interface{}) error {
	return c.ss.RecvMsg(m)
}
