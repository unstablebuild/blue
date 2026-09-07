// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
