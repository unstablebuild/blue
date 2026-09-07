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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type User struct {
	Role string
}

func loadKey(t *testing.T, filename string) Key {
	data, err := os.ReadFile(filename)
	require.NoError(t, err)

	var key Key
	if strings.HasSuffix(filename, ".pub") {
		key, err = LoadPublicKey(data)
	} else {
		key, err = LoadPrivateKey(data)
	}
	require.NoError(t, err)
	return key
}

func TestSignVerify(t *testing.T) {
	ecPub := loadKey(t, "./testdata/ecdh.pub")
	ecPriv := loadKey(t, "./testdata/ecdh.key")
	rsaPub := loadKey(t, "./testdata/rsa.pub")
	rsaPriv := loadKey(t, "./testdata/rsa.key")
	jwkPriv := loadKey(t, "./testdata/jwk-priv.json")
	jwkPub := loadKey(t, "./testdata/jwk-pub2.json.pub")
	strKey, err := SymmetricKey([]byte(symmetricKey))
	require.NoError(t, err)

	suite := []struct {
		description string
		keys        Keys
	}{
		// unsupported {"ecdh private to sign and verify", ecPriv, ecPriv},
		// unsupported {"rsa private to sign and verify", rsaPriv, rsaPriv},
		{"ecdh private to sign and public to verify", StaticAsymmetricKeys(ecPriv, ecPub)},
		{"rsa private to sign and public to verify", StaticAsymmetricKeys(rsaPriv, rsaPub)},
		{"jwk private to sign and public to verify", StaticAsymmetricKeys(jwkPriv, jwkPub)},
		{"symmetric key to sign and verify", StaticSymmetricKeys(strKey)},
	}

	for _, test := range suite {
		test := test
		t.Run(test.description, func(t *testing.T) {
			signKey, err := test.keys.Sign(context.Background())
			require.NoError(t, err)

			token, err := SignToken(signKey, "nada1234", "nada@unstable.build", User{Role: "admin"}, 1*time.Hour)
			require.NoError(t, err)

			verifyKeys, err := test.keys.Verify(context.Background())
			require.NoError(t, err)

			claims, err := VerifyToken[User](verifyKeys[0], token)
			require.NoError(t, err)

			assert.Equal(t, "nada1234", claims.UserID)
			assert.Equal(t, "nada@unstable.build", claims.Email)
			assert.Equal(t, "admin", claims.Extra.Role)
		})
	}
}
