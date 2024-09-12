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
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregate(t *testing.T) {
	suite := []struct {
		description string
		it          []Iterator[int]
		expectRes   []int
		expectErr   string
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
			expectErr: "oops",
		},
		{
			description: "returns last iterator error",
			it: []Iterator[int]{
				FromSlice[int]([]int{9}),
				Error[int](errors.New("oops")),
			},
			expectRes: []int{9},
			expectErr: "oops",
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
			if test.expectErr != "" {
				require.Error(t, actualErr)
				assert.True(t, strings.Contains(actualErr.Error(), test.expectErr))
			} else {
				assert.NoError(t, actualErr)
			}
		})
	}

	t.Run("Close is called as iterators are consumed", func(t *testing.T) {
		var (
			it1Closed bool
			it2Closed bool
		)
		var i int
		it1 := FromFunc[string](func() (string, bool, error) { return "1", i < 1, nil },
			func() error {
				it1Closed = true
				return nil
			})
		it2 := FromFunc[string](func() (string, bool, error) { return "2", i < 2, nil },
			func() error {
				it2Closed = true
				return nil
			})
		actualResIt := Aggregate[string](it1, it2)
		for ; i < 2; i++ {
			n, ok := actualResIt.Next()
			require.True(t, ok, i)
			assert.Equal(t, strconv.Itoa(i+1), n, i)
		}

		_, ok := actualResIt.Next()
		require.False(t, ok)

		assert.True(t, it1Closed)
		assert.True(t, it2Closed)

		it1Closed = false
		it2Closed = false
		require.NoError(t, actualResIt.Close())

		assert.False(t, it1Closed)
		assert.False(t, it2Closed)
	})

	t.Run("Close calls Close on all all iterators", func(t *testing.T) {
		var (
			it1Closed bool
			it2Closed bool
		)
		it1 := FromFunc[string](func() (string, bool, error) { return "1", true, nil },
			func() error {
				it1Closed = true
				return nil
			})
		it2 := FromFunc[string](func() (string, bool, error) { return "2", true, nil },
			func() error {
				it2Closed = true
				return nil
			})
		actualResIt := Aggregate[string](it1, it2)

		require.NoError(t, actualResIt.Close())

		assert.True(t, it1Closed)
		assert.True(t, it2Closed)
	})
}
