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

package cliformat

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
)

func TestTemplate(t *testing.T) {
	var ifc any //nolint:gosimple
	ifc = testStruct1{Public: "hello"}
	tsuite := []struct {
		desc        string
		inTemplate  string
		inEls       []any
		expectedOut string
		expectedErr bool
	}{
		{"empty iterator should print nothing",
			"{{ .Public }}", []any{}, "", false},
		{"non-object types should error (string)",
			"{{ .Public }}", []any{"one"}, "", true},
		{"non-object types should error (int)",
			"{{ .Public }}", []any{1}, "", true},
		{"non-object types should error (bool)",
			"{{ .Public }}", []any{true}, "", true},
		{"pass map by value",
			"{{ .Public }}", []any{map[string]any{"Public": "hello"}},
			"hello\n", false},
		{"pass struct by value",
			"{{ .Public }}", []any{testStruct1{Public: "hello"}},
			"hello\n", false},
		{"pass struct by ref",
			"{{ .Public }}", []any{&testStruct1{Public: "hello"}},
			"hello\n", false},
		{"pass struct by any",
			"{{ .Public }}", []any{ifc},
			"hello\n", false},
		{"preserve order of headers",
			"{{ .Public2 }} {{ .Public }}", []any{
				&testStruct1{Public: "x", Public2: 0},
				testStruct1{Public: "deux", Public2: 2},
				map[string]any{"Public": "trois", "Public2": "3"},
			},
			"0 x\n2 deux\n3 trois\n", false},
		{"private fields should error",
			"{{ .Public }} {{ .private }}", []any{&testStruct1{private: "bla", Public: ""}},
			` `, true},
		{"struct field",
			"{{ .Composite }}", []any{testStruct1{Composite: testStruct2{
				Map:      map[string]any{"Bootx": "Torn ACL"},
				SliceIfc: []any{"1", 1, true, false},
				SliceInt: []int{0},
			}}},
			"{map[Bootx:Torn ACL] [1 1 true false] [0]}\n", false},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			table, err := Template[any](tcase.inTemplate)
			require.NoError(t, err)
			var buf bytes.Buffer
			it := iterator.FromSlice(tcase.inEls)
			err = table.Format(context.Background(), &buf, it)
			if tcase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedOut, buf.String())
			}
		})
	}
}
