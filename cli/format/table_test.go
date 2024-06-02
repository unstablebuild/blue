package format

import (
	"bytes"
	"testing"

	"github.com/unstablebuild/blue/iterator"
	"github.com/stretchr/testify/assert"
)

type testStruct2 struct {
	Map      map[string]interface{}
	SliceIfc []interface{}
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
	var ifc interface{}
	ifc = testStruct1{Public: "hello"}
	tsuite := []struct {
		desc        string
		inFields    []string
		inEls       []interface{}
		expectedOut string
		expectedErr bool
	}{
		{"empty iterator should print nothing",
			[]string{}, []interface{}{}, "\n+\n+\n", false},
		{"non-object types should error (string)",
			[]string{}, []interface{}{"one"}, "", true},
		{"non-object types should error (int)",
			[]string{}, []interface{}{1}, "", true},
		{"non-object types should error (bool)",
			[]string{}, []interface{}{true}, "", true},
		{"pass map by value",
			[]string{"Public"}, []interface{}{map[string]interface{}{"Public": "hello"}},
			`
+--------+
| PUBLIC |
+--------+
| hello  |
+--------+
`, false},
		{"pass struct by value",
			[]string{"Public"}, []interface{}{testStruct1{Public: "hello"}},
			`
+--------+
| PUBLIC |
+--------+
| hello  |
+--------+
`, false},
		{"pass struct by ref",
			[]string{"Public"}, []interface{}{&testStruct1{Public: "hello"}},
			`
+--------+
| PUBLIC |
+--------+
| hello  |
+--------+
`, false},
		{"pass struct by interface{}",
			[]string{"Public"}, []interface{}{ifc},
			`
+--------+
| PUBLIC |
+--------+
| hello  |
+--------+
`, false},
		{"preserve order of headers",
			[]string{"Public2", "Public"}, []interface{}{
				&testStruct1{Public: "x", Public2: 0},
				testStruct1{Public: "deux", Public2: 2},
				map[string]interface{}{"Public": "trois", "Public2": "3"},
			},
			`
+---------+--------+
| PUBLIC2 | PUBLIC |
+---------+--------+
|       0 | x      |
|       2 | deux   |
|       3 | trois  |
+---------+--------+
`, false},
		{"private fields should not be printed",
			[]string{"Public", "private"}, []interface{}{&testStruct1{private: "bla", Public: ""}},
			`
+--------+---------+
| PUBLIC | PRIVATE |
+--------+---------+
|        |         |
+--------+---------+
`, false},
		{"struct field",
			[]string{"Composite"}, []interface{}{testStruct1{Composite: testStruct2{
				Map:      map[string]interface{}{"Bootx": "Torn ACL"},
				SliceIfc: []interface{}{"1", 1, true, false},
				SliceInt: []int{0},
			}}},
			`
+--------------------------------+
|           COMPOSITE            |
+--------------------------------+
| {map[Bootx:Torn ACL] [1 1 true |
| false] [0]}                    |
+--------------------------------+
`, false},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			table := Table[interface{}](tcase.inFields)
			var buf bytes.Buffer
			buf.WriteString("\n") // make test cases easier to write
			it := iterator.FromSlice[any](tcase.inEls)
			err := table.Format(&buf, it)
			if tcase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tcase.expectedOut, buf.String())
			}
		})
	}
}
