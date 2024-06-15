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

func TestRetryErrorValue(t *testing.T) {
	t.Run("returns error as is if retry was set to false since the start", func(t *testing.T) {
		origErr := errors.New("bla")
		err := Retry(context.Background(), LimitStrategy(2), func(ctx context.Context) (bool, error) {
			return false, origErr
		})
		assert.Equal(t, origErr, err)
	})
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

	// start running when cancelCtx is guaranteed to actually have been cancelled
	// so test cases that employ canceledCtx are deterministic
	<-canceledCtx.Done()

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
