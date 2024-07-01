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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReduce(t *testing.T) {
	suite := []struct {
		description string
		it          Iterator[int]
		fn          func(int, int) (int, error)
		expectRes   int
		expectErr   error
	}{
		{
			description: "empty iterator returns zero value",
			it:          Empty[int](),
			fn:          func(int, int) (int, error) { return 0, nil },
			expectRes:   0,
		},
		{
			description: "iterator with single value returns that value",
			it:          FromSlice[int]([]int{9}),
			fn:          func(ret int, i int) (int, error) { return ret + i, nil },
			expectRes:   9,
		},
		{
			description: "returns fn error",
			it:          FromSlice[int]([]int{9}),
			fn:          func(ret int, i int) (int, error) { return 0, errors.New("oops") },
			expectErr:   errors.New("oops"),
		},
		{
			description: "continues calling fn until iterator is exhausted",
			it:          FromSlice[int]([]int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}),
			fn:          func(ret int, i int) (int, error) { return ret + i, nil },
			expectRes:   10,
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			actualRes, actualErr := Reduce(test.it, test.fn)
			assert.Equal(t, test.expectRes, actualRes)
			assert.Equal(t, test.expectErr, actualErr)
		})
	}
}
