package format

import (
	"bytes"
	"testing"

	"github.com/ernestrc/blue/iterator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplate(t *testing.T) {
	var ifc interface{}
	ifc = testStruct1{Public: "hello"}
	tsuite := []struct {
		desc        string
		inTemplate string
		inEls       []interface{}
		expectedOut string
		expectedErr bool
	}{
		{"empty iterator should print nothing",
			"{{ .Public }}", []interface{}{}, "", false},
		{"non-object types should error (string)",
			"{{ .Public }}", []interface{}{"one"}, "", true},
		{"non-object types should error (int)",
			"{{ .Public }}", []interface{}{1}, "", true},
		{"non-object types should error (bool)",
			"{{ .Public }}", []interface{}{true}, "", true},
		{"pass map by value",
			"{{ .Public }}", []interface{}{map[string]interface{}{"Public": "hello"}},
			"hello\n", false},
		{"pass struct by value",
			"{{ .Public }}", []interface{}{testStruct1{Public: "hello"}},
			"hello\n", false},
		{"pass struct by ref",
			"{{ .Public }}", []interface{}{&testStruct1{Public: "hello"}},
			"hello\n", false},
		{"pass struct by interface{}",
			"{{ .Public }}", []interface{}{ifc},
			"hello\n", false},
		{"preserve order of headers",
			"{{ .Public2 }} {{ .Public }}", []interface{}{
				&testStruct1{Public: "x", Public2: 0},
				testStruct1{Public: "deux", Public2: 2},
				map[string]interface{}{"Public": "trois", "Public2": "3"},
			},
			"0 x\n2 deux\n3 trois\n", false},
		{"private fields should error",
			"{{ .Public }} {{ .private }}", []interface{}{&testStruct1{private: "bla", Public: ""}},
			` `, true},
		{"struct field",
			"{{ .Composite }}", []interface{}{testStruct1{Composite: testStruct2{
				Map:      map[string]interface{}{"Bootx": "Torn ACL"},
				SliceIfc: []interface{}{"1", 1, true, false},
				SliceInt: []int{0},
			}}},
			"{map[Bootx:Torn ACL] [1 1 true false] [0]}\n", false},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			table, err  := Template[interface{}](tcase.inTemplate)
			require.NoError(t, err)
			var buf bytes.Buffer
			it := iterator.FromSlice[any](tcase.inEls)
			err = table.Format(&buf, it)
			if tcase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedOut, buf.String())
			}
		})
	}
}
