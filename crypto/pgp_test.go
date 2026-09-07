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

package crypto

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/crypto/cryptotest"
)

func TestSignVerify(t *testing.T) {
	t.Run("verifies signature correctly", func(t *testing.T) {
		key := Key(cryptotest.GenerateTestKey(t))

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, key)
		require.NoError(t, err)

		err = Verify(strings.NewReader(str), &out, key)
		assert.NoError(t, err)
	})

	t.Run("returns error if data has been tampered with", func(t *testing.T) {
		key := Key(cryptotest.GenerateTestKey(t))

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, key)
		require.NoError(t, err)

		str = "bluectl cryp\x00"
		err = Verify(strings.NewReader(str), &out, key)
		assert.Error(t, err)
	})

	t.Run("returns error if a different key pair has been used to sign data", func(t *testing.T) {
		key1 := Key(cryptotest.GenerateTestKey(t))
		key2 := Key(cryptotest.GenerateTestKey(t))

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, key1)
		require.NoError(t, err)

		err = Verify(strings.NewReader(str), &out, key2)
		assert.Error(t, err)
	})
}
