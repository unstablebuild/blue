package grpc

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
