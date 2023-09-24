package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombineKeys(t *testing.T) {
	t.Run("with two args", func(t *testing.T) {
		testSignKey1 := SymmetricKey([]byte("1234"))
		testSignKeys1 := StaticSymmetricKeys(testSignKey1)
		testSignKey2 := SymmetricKey([]byte("1235"))
		testSignKeys2 := StaticSymmetricKeys(testSignKey2)
		combined := CombineKeys(testSignKeys1, testSignKeys2)

		t.Run("Sign returns the first key arg", func(t *testing.T) {
			k, err := combined.Sign(context.Background())
			require.NoError(t, err)
			assert.Equal(t, testSignKey1, k)
		})

		t.Run("Verify returns all the keys", func(t *testing.T) {
			keys, err := combined.Verify(context.Background())
			require.NoError(t, err)
			assert.ElementsMatch(t, []Key{testSignKey1, testSignKey2}, keys)
		})
	})
	t.Run("with one key", func(t *testing.T) {
		testSignKey1 := SymmetricKey([]byte("1234"))
		testSignKeys1 := StaticSymmetricKeys(testSignKey1)
		combined := CombineKeys(testSignKeys1)

		t.Run("Sign returns the first key arg", func(t *testing.T) {
			k, err := combined.Sign(context.Background())
			require.NoError(t, err)
			assert.Equal(t, testSignKey1, k)
		})

		t.Run("Verify returns all the keys", func(t *testing.T) {
			keys, err := combined.Verify(context.Background())
			require.NoError(t, err)
			assert.ElementsMatch(t, []Key{testSignKey1}, keys)
		})
	})
}
