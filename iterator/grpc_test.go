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
