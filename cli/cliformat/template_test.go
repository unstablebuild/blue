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
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
)

func TestTemplate(t *testing.T) {
	var ifc any = testStruct1{Public: "hello"}
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
