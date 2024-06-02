package context

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
		fctx := First(ctx)

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
		fctx := First(context.Background(), context.TODO())

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

		fctx := First(ctx1, ctx2, ctx3)

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

		fctx := First(ctx1, ctx2, ctx3)

		actualValue := fctx.Value("a")
		assert.Equal(t, "A", actualValue)

		actualValue = fctx.Value("b")
		assert.Equal(t, "B", actualValue)
	})

	t.Run("only one returns the one's value's, deadline, done ctx", func(t *testing.T) {
		ctx1, cancel1 := context.WithCancel(context.Background())
		ctx2, cancel2 := context.WithCancel(context.Background())
		ctx3, cancel3 := context.WithCancel(context.Background())
		fctx := First(ctx1, ctx2, ctx3)

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
