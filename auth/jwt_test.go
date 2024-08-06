// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
	strKey := SymmetricKey([]byte("1234"))

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
