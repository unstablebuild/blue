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
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

type data struct {
	X string
}

func TestStreamIterator(t *testing.T) {
	t.Run("Double Close is a no-op", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		ctx, cancel := context.WithCancel(context.Background())
		it := FromRawStream[data](ctx, cancel, &errorStream{err: errors.New("oops")})

		_, ok := it.Next(ctx)
		assert.False(t, ok)
		require.NoError(t, it.Close())
		require.NoError(t, it.Close())
	})

	t.Run("unblocks Next if context is canceled", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		stream := blockingStream{quitCh: make(chan struct{})}
		it := FromRawStream[data](ctx, func() {
			close(stream.quitCh)
			cancel()
		}, stream)
		_, ok := it.Next(ctx)
		assert.False(t, ok)
		assert.Error(t, it.Err())

		require.NoError(t, it.Close())
	})

	t.Run("unblocks Next if Close is called", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		ctx, cancel := context.WithCancel(context.Background())
		stream := blockingStream{quitCh: make(chan struct{})}
		it := FromRawStream[data](ctx, func() {
			close(stream.quitCh)
			cancel()
		}, stream)
		go func() {
			_ = it.Close()
		}()
		_, ok := it.Next(ctx)
		assert.False(t, ok)
	})

	t.Run("exhausts underlying document iterator", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		a, b, c := data{X: "a"}, data{X: "b"}, data{X: "c"}
		stream := grpcClientStream{data: []data{a, b, c}, ready: make(chan struct{}, 3)}
		var cancelCalled bool
		ctx, cancel := context.WithCancel(context.Background())
		it := FromRawStream[data](ctx, func() {
			cancelCalled = true
			cancel()
		}, &stream)
		actual, err := Reduce(context.Background(), it,
			func(ret []data, t *data) ([]data, error) {
				return append(ret, *t), nil
			})
		require.NoError(t, err)
		assert.EqualValues(t, []data{a, b, c}, actual)

		err = it.Close()
		require.NoError(t, err)
		assert.True(t, cancelCalled)

		require.NoError(t, it.Close())
	})

	t.Run("bubbles up underlying document iterator errors", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		ctx, cancel := context.WithCancel(context.Background())
		it := FromRawStream[data](ctx, cancel, &errorStream{err: errors.New("kaboom")})
		_, ok := it.Next(ctx)
		assert.False(t, ok)
		assert.EqualError(t, it.Err(), "1 error occurred: kaboom")
		require.NoError(t, it.Close())
	})

	t.Run("handles wrapped io.EOF errors", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		ctx, cancel := context.WithCancel(context.Background())
		it := FromRawStream[data](ctx, cancel, &errorStream{
			err: fmt.Errorf("something: %w", io.EOF),
		})
		_, ok := it.Next(ctx)
		assert.False(t, ok)
		assert.NoError(t, it.Err())
		require.NoError(t, it.Close())
	})

	t.Run("does not leak goroutine if data was read, "+
		"but Close was called before Next", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		a, b, c := data{X: "a"}, data{X: "b"}, data{X: "c"}
		ready := make(chan struct{}) // blocking
		stream := grpcClientStream{data: []data{a, b, c}, ready: ready}
		ctx, cancel := context.WithCancel(context.Background())
		it := FromRawStream[data](ctx, cancel, &stream)
		<-ready

		require.NoError(t, it.Close())
	})

	t.Run("sends acks", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		a, b, c := data{X: "a"}, data{X: "b"}, data{X: "c"}
		stream := generatedStream{data: []data{a, b, c}}
		ctx, cancel := context.WithCancel(context.Background())
		it := FromStreamWithAck[data, data](ctx, cancel, &stream)

		values, err := ToSlice(context.Background(), it)
		require.NoError(t, err)

		assert.EqualValues(t, []*data{&a, &b, &c}, values)
		assert.EqualValues(t, []*data{new(data), new(data), new(data)}, stream.send)
	})

	t.Run("bubbles up ack errors", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		a, b, c := data{X: "a"}, data{X: "b"}, data{X: "c"}
		stream := generatedStream{data: []data{a, b, c}, err: errors.New("Janis")}
		ctx, cancel := context.WithCancel(context.Background())
		it := FromStreamWithAck[data, data](ctx, cancel, &stream)

		_, ok := it.Next(ctx)
		require.False(t, ok)
		require.EqualError(t, it.Err(), "1 error occurred: Janis")
	})
}

type generatedStream struct {
	err  error
	data []data
	send []*data
}

func (b *generatedStream) Send(d *data) error {
	b.send = append(b.send, d)
	return b.err
}

func (b *generatedStream) Recv() (*data, error) {
	if len(b.data) == 0 {
		return nil, io.EOF
	}
	ptr := new(data)
	*ptr = b.data[0]
	b.data = b.data[1:]
	return ptr, b.err
}

type grpcClientStream struct {
	data  []data
	ready chan struct{}
}

func (b *grpcClientStream) RecvMsg(doc interface{}) error {
	if len(b.data) == 0 {
		return io.EOF
	}
	ptr := doc.(*data)
	*ptr = b.data[0]
	b.data = b.data[1:]
	b.ready <- struct{}{}
	return nil
}

type blockingStream struct {
	quitCh chan struct{}
}

func (b blockingStream) RecvMsg(doc interface{}) error {
	<-b.quitCh
	return nil
}

type errorStream struct {
	err error
}

func (b *errorStream) RecvMsg(doc interface{}) error {
	return b.err
}
