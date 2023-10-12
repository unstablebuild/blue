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
}
