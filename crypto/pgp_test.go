package crypto

import (
	"bytes"
	"strings"
	"testing"

	cryptest "github.com/ernestrc/blue/crypto/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignVerify(t *testing.T) {
	t.Run("verifies signature correctly", func(t *testing.T) {
		key := Key(cryptest.GenerateTestKey(t))

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, key)
		require.NoError(t, err)

		err = Verify(strings.NewReader(str), &out, key)
		assert.NoError(t, err)
	})

	t.Run("returns error if data has been tampered with", func(t *testing.T) {
		key := Key(cryptest.GenerateTestKey(t))

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, key)
		require.NoError(t, err)

		str = "bluectl cryp\x00"
		err = Verify(strings.NewReader(str), &out, key)
		assert.Error(t, err)
	})

	t.Run("returns error if a different key pair has been used to sign data", func(t *testing.T) {
		key1 := Key(cryptest.GenerateTestKey(t))
		key2 := Key(cryptest.GenerateTestKey(t))

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, key1)
		require.NoError(t, err)

		err = Verify(strings.NewReader(str), &out, key2)
		assert.Error(t, err)
	})
}
