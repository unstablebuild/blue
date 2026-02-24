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
	"github.com/unstablebuild/blue/iterator"
)

type testStruct2 struct {
	Map      map[string]any
	SliceIfc []any
	SliceInt []int
}

type testStruct1 struct {
	Public    string
	Public2   int
	private   string
	Composite testStruct2
}

func TestTable(t *testing.T) {
	//nolint:gosimple
	var ifc any = testStruct1{Public: "hello"}
	tsuite := []struct {
		desc        string
		inFields    []string
		inEls       []any
		expectedOut string
		expectedErr bool
	}{
		{"empty iterator should print nothing",
			[]string{}, []any{}, "\n", false},
		{"non-object types should error (string)",
			[]string{}, []any{"one"}, "", true},
		{"non-object types should error (int)",
			[]string{}, []any{1}, "", true},
		{"non-object types should error (bool)",
			[]string{}, []any{true}, "", true},
		{"pass map by value",
			[]string{"Public"}, []any{map[string]any{"Public": "hello"}},
			`
┌────────┐
│ PUBLIC │
├────────┤
│ hello  │
└────────┘
`, false},
		{"pass struct by value",
			[]string{"Public"}, []any{testStruct1{Public: "hello"}},
			`
┌────────┐
│ PUBLIC │
├────────┤
│ hello  │
└────────┘
`, false},
		{"pass struct by ref",
			[]string{"Public"}, []any{&testStruct1{Public: "hello"}},
			`
┌────────┐
│ PUBLIC │
├────────┤
│ hello  │
└────────┘
`, false},
		{"pass struct by any",
			[]string{"Public"}, []any{ifc},
			`
┌────────┐
│ PUBLIC │
├────────┤
│ hello  │
└────────┘
`, false},
		{"preserve order of headers",
			[]string{"Public2", "Public"}, []any{
				&testStruct1{Public: "x", Public2: 0},
				testStruct1{Public: "deux", Public2: 2},
				map[string]any{"Public": "trois", "Public2": "3"},
			},
			`
┌──────────┬────────┐
│ PUBLIC 2 │ PUBLIC │
├──────────┼────────┤
│ 0        │ x      │
│ 2        │ deux   │
│ 3        │ trois  │
└──────────┴────────┘
`, false},
		{"private fields should not be printed",
			[]string{"Public", "private"}, []any{&testStruct1{private: "bla", Public: ""}},
			`
┌────────┬─────────┐
│ PUBLIC │ PRIVATE │
├────────┼─────────┤
│        │         │
└────────┴─────────┘
`, false},
		{"struct field",
			[]string{"Composite"}, []any{testStruct1{Composite: testStruct2{
				Map:      map[string]any{"Bootx": "Torn ACL"},
				SliceIfc: []any{"1", 1, true, false},
				SliceInt: []int{0},
			}}},
			`
┌────────────────────────────────────────────┐
│                 COMPOSITE                  │
├────────────────────────────────────────────┤
│ {map[Bootx:Torn ACL] [1 1 true false] [0]} │
└────────────────────────────────────────────┘
`, false},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			table := Table[any](tcase.inFields)
			var buf bytes.Buffer
			buf.WriteString("\n") // make test cases easier to write
			it := iterator.FromSlice(tcase.inEls)
			err := table.Format(context.Background(), &buf, it)
			if tcase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedOut, buf.String())
			}
		})
	}
}
