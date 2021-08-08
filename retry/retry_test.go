package retry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func succeedAfter(n int) func(context.Context) (bool, error) {
	return func(context.Context) (bool, error) {
		if n == 0 {
			return false, nil
		}
		n--
		return true, errors.New("whoopsie")
	}
}

func failAfter(n int) func(context.Context) (bool, error) {
	return func(context.Context) (bool, error) {
		if n == 0 {
			return false, errors.New("last one")
		}
		n--
		return true, errors.New("whoopsie")
	}
}

var canceledCtx, cancel = context.WithCancel(context.Background())

func init() {
	cancel()
}

func TestRetry(t *testing.T) {
	tsuite := []struct {
		msg       string
		ctx       context.Context
		strategy  Strategy
		wantCalls int
		wantErr   string
		fn        func(ctx context.Context) (bool, error)
	}{
		{"succeeds on first attempt calls fn only once",
			context.Background(), LimitStrategy(2), 1, "", succeedAfter(0)},
		{"succeeds on last attempt calls fn twice",
			context.Background(), LimitStrategy(2), 2, "", succeedAfter(1)},
		{"fails on last strategy attempt calls fn 3",
			context.Background(), LimitStrategy(2), 2,
			"2 errors occurred: whoopsie; whoopsie", succeedAfter(2)},
		{"fails on last function attempt calls fn 3",
			context.Background(), LimitStrategy(3), 3,
			"3 errors occurred: whoopsie; whoopsie; last one", failAfter(2)},
		{"fails on context deadline error calls fn 1",
			canceledCtx, LimitStrategy(3), 1,
			"2 errors occurred: context canceled; whoopsie", succeedAfter(2)},
	}

	for _, tcase := range tsuite {
		tcase := tcase
		t.Run(tcase.msg, func(t *testing.T) {
			var calls int
			err := Retry(tcase.ctx, tcase.strategy, func(ctx context.Context) (bool, error) {
				calls++
				return tcase.fn(ctx)
			})
			if tcase.wantErr != "" {
				assert.EqualError(t, err, tcase.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tcase.wantCalls, calls)
		})
	}
}
