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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnslice(t *testing.T) {
	suite := []struct {
		description string
		it          Iterator[[]int]
		expectRes   []int
	}{
		{
			description: "empty iterator returns empty iterator",
			it:          Empty[[]int](),
			expectRes:   nil,
		},
		{
			description: "iterator with single value returns that value",
			it:          FromSlice[[]int]([][]int{{9}}),
			expectRes:   []int{9},
		},
		{
			description: "iterator with multiple slices with different lengths",
			it:          FromSlice[[]int]([][]int{{1, 1}, {1}, {1}, {1, 1, 1, 1}, {1, 1}}),
			expectRes:   []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		},
		{
			description: "continues calling fn until iterator is exhausted",
			it:          FromSlice[[]int]([][]int{{1, 1}, nil, {1, 1}, {1, 1, 1, 1}, nil, nil, {1, 1}}),
			expectRes:   []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			actualResIt := Unslice(test.it)
			actualRes, err := Reduce(actualResIt, func(ret []int, i int) ([]int, error) {
				return append(ret, i), nil
			})
			require.NoError(t, err)
			assert.Equal(t, test.expectRes, actualRes)
		})
	}
}
