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

package iterator

import (
	"context"
	"io"
	"sync/atomic"
)

// Stream abstract a GRPC protoc-generated grpc.ClientStream.
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
			if err == io.EOF {
				ok = false
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
