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

func TestIsEmpty(t *testing.T) {
	tsuite := []struct {
		desc          string
		inSlice       []testStruct
		expectedOutOk bool
	}{
		{"empty returns true and empty iterator", nil, true},
		{"one item returns false and and same item iterator", []testStruct{{"1", 1}}, false},
		{"multiple items returns false and and same iterator", []testStruct{{"1", 1}, {"2", 2}}, false},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			actualOutIt, actualOutOk := IsEmpty(FromSlice(tcase.inSlice))
			assert.Equal(t, tcase.expectedOutOk, actualOutOk)
			actualOutSlice, err := ToSlice(actualOutIt)
			require.NoError(t, err)
			assert.Equal(t, append([]testStruct{}, tcase.inSlice...), actualOutSlice)
		})
	}

	t.Run("iterator returned Empty is empty", func(t *testing.T) {
		next, empty := IsEmpty(Empty[string]())
		require.True(t, empty)

		_, empty = IsEmpty(next)
		require.True(t, empty)
	})
}
