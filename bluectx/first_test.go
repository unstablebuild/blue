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

package bluectx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestFirt(t *testing.T) {
	t.Run("only one returns the one's value's, deadline, done ctx", func(t *testing.T) {
		tt := time.Now().Add(24 * time.Hour)

		ctx := context.Background()
		//nolint:staticcheck
		ctx = context.WithValue(ctx, "a", "A")
		ctx, cancel := context.WithDeadline(ctx, tt)
		fctx, cancelf := First(ctx)
		defer cancelf()

		actualValue := fctx.Value("a")
		assert.Equal(t, "A", actualValue)

		d, ok := fctx.Deadline()
		assert.True(t, ok)
		assert.Equal(t, tt, d)

		ch := fctx.Done()
		select {
		case <-ch:
			require.True(t, false, "done was canceled already")
		default:
		}

		assert.NoError(t, fctx.Err())

		cancel()
		ch = fctx.Done()
		select {
		case <-ch:
			assert.Error(t, fctx.Err())
		default:
			require.True(t, false, "done was not canceled correctly")
		}
	})

	t.Run("returns false if none have a deadline set", func(t *testing.T) {
		fctx, cancelf := First(context.Background(), context.TODO())
		defer cancelf()

		_, ok := fctx.Deadline()
		assert.False(t, ok)
	})

	t.Run("multiple return the first deadline", func(t *testing.T) {
		tt1 := time.Now().Add(24 * time.Hour)
		tt2 := time.Now().Add(14 * time.Hour)
		tt3 := time.Now().Add(22 * time.Hour)

		ctx1, cancel1 := context.WithDeadline(context.Background(), tt1)
		defer cancel1()
		ctx2, cancel2 := context.WithDeadline(context.Background(), tt2)
		defer cancel2()
		ctx3, cancel3 := context.WithDeadline(context.Background(), tt3)
		defer cancel3()

		fctx, cancelf := First(ctx1, ctx2, ctx3)
		defer cancelf()

		d, ok := fctx.Deadline()
		assert.True(t, ok)
		assert.Equal(t, tt2, d)
	})

	t.Run("multiple return the first value", func(t *testing.T) {
		ctx1, cancel := context.WithCancel(context.Background()) // avoid leak
		defer cancel()
		// nolint:staticcheck
		ctx2 := context.WithValue(context.Background(), "a", "A")
		// nolint:staticcheck
		ctx3 := context.WithValue(context.Background(), "a", "B")
		// nolint:staticcheck
		ctx3 = context.WithValue(ctx3, "b", "B")

		fctx, cancelf := First(ctx1, ctx2, ctx3)
		defer cancelf()

		actualValue := fctx.Value("a")
		assert.Equal(t, "A", actualValue)

		actualValue = fctx.Value("b")
		assert.Equal(t, "B", actualValue)
	})

	t.Run("only one returns the one's value's, deadline, done ctx", func(t *testing.T) {
		ctx1, cancel1 := context.WithCancel(context.Background())
		ctx2, cancel2 := context.WithCancel(context.Background())
		ctx3, cancel3 := context.WithCancel(context.Background())
		fctx, cancelf := First(ctx1, ctx2, ctx3)
		defer cancelf()

		defer cancel2()
		defer cancel3()

		_, ok := fctx.Deadline()
		assert.False(t, ok)

		ch := fctx.Done()
		select {
		case <-ch:
			require.True(t, false, "done was canceled already")
		default:
		}

		assert.NoError(t, fctx.Err())

		cancel1()
		ch = fctx.Done()
		select {
		case <-ch:
			assert.Error(t, fctx.Err())
		default:
			require.True(t, false, "done was not canceled correctly")
		}
	})

	goleak.VerifyNone(t)
}
