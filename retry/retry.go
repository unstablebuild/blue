package retry

import (
	"context"
	"math"
	"time"

	multierror "github.com/ernestrc/go-multierror"
)

// Strategy represents a retry strategy. See Retry for more details.
type Strategy func(count uint) (sleep time.Duration, stop bool)

// DefaultStrategy returns an exponential backoff strategy with a limit of 10, a min sleep
// time of 1ms and a max sleep time of 1024ms. At most, this strategy yields a retry mechanism
// that can last approximately 2 seconds.
var DefaultStrategy Strategy = CombinedStrategy(
	LimitStrategy(10), ExponentialStrategy(1*time.Millisecond, 1024*time.Millisecond))

// LimitStrategy returns a retry strategy that stops after limit number of retries
// has been reached.
func LimitStrategy(limit uint) Strategy {
	return func(count uint) (sleep time.Duration, stop bool) {
		return 0, count >= limit
	}
}

// ExponentialStrategy returns a retry strategy that never stops but increments
// the sleep time from min to max in power of 2 increments.
func ExponentialStrategy(min, max time.Duration) Strategy {
	return func(count uint) (sleep time.Duration, stop bool) {
		sleep = time.Duration(math.Pow(2, float64(count-1))) * min
		if sleep > max {
			sleep = max
		}
		return
	}
}

// SequentialStrategy returns a retry strategy that never stops and retries
// always after 'every' duration.
func SequentialStrategy(every time.Duration) Strategy {
	return func(count uint) (sleep time.Duration, stop bool) {
		sleep = every
		return
	}
}

// CombinedStrategy returns a retry strategy that combines all the given strategies
// using the following rules:
//   - If any returns stop=true, then stop=true is returned.
//   - If multiple return a sleep time that is non-zero, then the biggest sleep value is used.
func CombinedStrategy(i Strategy, n ...Strategy) Strategy {
	all := append([]Strategy{}, i)
	all = append(all, n...)
	return func(count uint) (sleep time.Duration, stop bool) {
		var max time.Duration
		for _, strategy := range all {
			sleep, stop = strategy(count)
			if stop {
				return
			}
			if sleep > max {
				max = sleep
			}
		}
		return max, false
	}
}

// Retry retries the given function with the given Strategy, until function returns
// no error, strategy returns stop=true, function returns error and retry=false,
// or ctx deadline is exceeded.
func Retry(
	ctx context.Context, strategy Strategy,
	fn func(ctx context.Context) (bool, error),
) error {
	var retryCount uint
	var result error
	for {
		retry, err := fn(ctx)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			result = multierror.Append(result, ctx.Err())
		}
		if !retry {
			// if we never allowed retries, pass error as is
			if result == nil {
				return err
			}
			return multierror.Append(result, err)
		}
		result = multierror.Append(result, err)

		retryCount++
		sleep, stop := strategy(retryCount)
		if stop {
			return result
		}
		select {
		case <-ctx.Done():
			return result
		case <-time.After(sleep):
		}
	}
}
