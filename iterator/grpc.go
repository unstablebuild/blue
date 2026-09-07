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

package iterator

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
)

// Stream abstract a GRPC protoc-generated wrapper of grpc.ClientStream.
type Stream[T any] interface {
	// Recv blocks until it receives a message into m or the stream is
	// done. It returns io.EOF when the stream completes successfully. On
	// any other error, the stream is aborted and the error contains the RPC
	// status.
	Recv() (*T, error)
}

// RawStream abstract a subset of grpc.ClientStream.
type RawStream interface {
	// RecvMsg blocks until it receives a message into m or the stream is
	// done. It returns io.EOF when the stream completes successfully. On
	// any other error, the stream is aborted and the error contains the RPC
	// status.
	RecvMsg(m interface{}) error
}

// BidiStream abstract a GRPC a protoc-generated wrapper of
// grpc.ClientStream with bidirectional communication capabilities.
type BidiStream[T any, A any] interface {
	Stream[T]
	// Send sends a message of type A. See grpc.ClientStream.SendMsg for
	// more details.
	Send(*A) error
}

// ValueStream abstract an arbitrary value stream. See Stream for differences.
type ValueStream[T any] interface {
	// Recv blocks until it receives a message into m or the stream is
	// done. It returns io.EOF when the stream completes successfully. On
	// any other error, the stream is aborted and the error contains the RPC
	// status.
	Recv() (T, error)
}

// FromStream takes a Stream, tipically genereated via protoc, and
// returns an Iterator of T. The iterator stops when its Close function is returned or
// the underlying stream returns io.EOF or other error. Only non io.EOF errors
// will be surfaced via the returning iterator's Err method.
//
// The given cancel function should be the context canceling function fed to
// the grpc.ClientStream's constructor. This is to ensure that when the
// returned iterator's Close method is called, the stream's resources
// are also cleaned up.
func FromStream[T any](
	ctx context.Context, cancel func(), stream Stream[T],
) Iterator[*T] {
	return FromValueStream[*T](ctx, cancel, stream)
}

// FromStreamWithAck returns an iterator of T that sends back an ack message
// of type A for every chunk of T received. See FromRawStream for more details.
func FromStreamWithAck[T any, A any](
	ctx context.Context, cancel func(), stream BidiStream[T, A],
) Iterator[*T] {
	ack := new(A)
	return fromValueStreamWithAck[*T, BidiStream[T, A]](
		ctx, cancel, stream, func(stream BidiStream[T, A]) error {
			return stream.Send(ack)
		})
}

// FromRawStream works similar to FromStream, but takes a raw grpc.ClientStream,
// so it's inherently less safe than using FromStream.
func FromRawStream[T any](
	ctx context.Context, cancel func(), stream RawStream,
) Iterator[*T] {
	return FromStream[T](ctx, cancel, rawStreamAdapter[T]{stream})
}

// FromValueStream works similar to FromStream, but expects a ValueStream.
// See ValueStream for more details.
func FromValueStream[T any](
	ctx context.Context, cancel func(), stream ValueStream[T],
) Iterator[T] {
	return fromValueStreamWithAck[T, ValueStream[T]](
		ctx, cancel, stream, nil)
}

func fromValueStreamWithAck[T any, S ValueStream[T]](
	ctx context.Context, cancel func(), stream S,
	ack func(stream S) error,
) Iterator[T] {
	var closed atomic.Bool
	type msg struct {
		data T
		err  error
	}

	ch := make(chan msg)
	go func() {
		defer close(ch)
		for {
			data, err := stream.Recv()
			select {
			case ch <- msg{data: data, err: err}:
				if err != nil {
					return
				}
				if ack == nil {
					continue
				}
				err := ack(stream)
				if err != nil {
					select {
					case ch <- msg{err: err}:
					case <-ctx.Done():
					}
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return FromFunc(func(ctx context.Context) (ret T, ok bool, err error) {
		var m msg
		select {
		case m, ok = <-ch:
			ret = m.data
			err = m.err
			if err != nil {
				ok = false
			}
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return
		case <-ctx.Done():
			err = ctx.Err()
			return
		}
	}, func() error {
		if !closed.CompareAndSwap(false, true) {
			return nil
		}
		cancel()
		<-ch // wait for clean goroutine to be done
		return nil
	})
}

type rawStreamAdapter[T any] struct {
	s RawStream
}

func (r rawStreamAdapter[T]) Recv() (*T, error) {
	ret := new(T)
	err := r.s.RecvMsg(ret)
	return ret, err
}
