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

func TestAggregate(t *testing.T) {
	suite := []struct {
		description string
		it          []Iterator[int]
		expectRes   []int
		expectErr   error
	}{
		{
			description: "no iterators returns empty iterator",
			it:          []Iterator[int]{},
			expectRes:   nil,
		},
		{
			description: "one empty iterator returns empty iterator",
			it:          []Iterator[int]{Empty[int]()},
			expectRes:   nil,
		},
		{
			description: "iterator with single value returns that value",
			it:          []Iterator[int]{FromSlice[int]([]int{9})},
			expectRes:   []int{9},
		},
		{
			description: "returns first iterator error",
			it: []Iterator[int]{
				Error[int](errors.New("oops")),
				FromSlice[int]([]int{9}),
			},
			expectErr: errors.New("oops"),
		},
		{
			description: "returns last iterator error",
			it: []Iterator[int]{
				FromSlice[int]([]int{9}),
				Error[int](errors.New("oops")),
			},
			expectRes: []int{9},
			expectErr: errors.New("oops"),
		},
		{
			description: "continues calling fn until iterator is exhausted",
			it: []Iterator[int]{
				FromSlice[int]([]int{2, 2}),
				Empty[int](),
				FromSlice[int]([]int{1}),
				FromSlice[int](nil),
			},
			expectRes: []int{2, 2, 1},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			actualResIt := Aggregate[int](test.it...)
			actualRes, actualErr := Reduce(actualResIt, func(ret []int, i int) ([]int, error) {
				return append(ret, i), nil
			})
			assert.Equal(t, test.expectRes, actualRes)
			assert.Equal(t, test.expectErr, actualErr)
		})
	}
}
