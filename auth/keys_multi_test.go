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

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const symmetricKey = "12345678901234567890123456789012"

func TestCombineKeys(t *testing.T) {
	t.Run("with two args", func(t *testing.T) {
		testSignKey1, err := SymmetricKey([]byte(symmetricKey))
		require.NoError(t, err)
		testSignKeys1 := StaticSymmetricKeys(testSignKey1)
		testSignKey2, err := SymmetricKey([]byte(symmetricKey + "1"))
		require.NoError(t, err)
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
		testSignKey1, err := SymmetricKey([]byte(symmetricKey))
		require.NoError(t, err)
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
