package document

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBolt(t *testing.T) {
	testDatastore(t, func(t *testing.T) Service {
		f, err := ioutil.TempFile("", "barnack_bolt_test")
		require.NoError(t, err)
		defer f.Close()

		store, err := NewBolt(f.Name(), "test")
		require.NoError(t, err)

		return store
	})
}
