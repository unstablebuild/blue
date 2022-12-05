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
		{"empty returns false and empty iterator", nil, false},
		{"one item returns true and and same item iterator", []testStruct{{"1", 1}}, true},
		{"multiple items returns true and and same iterator", []testStruct{{"1", 1}, {"2", 2}}, true},
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
}
