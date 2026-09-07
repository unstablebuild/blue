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
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchPublicJWKS(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &privateKey.PublicKey,
		Algorithm: string(jose.RS256),
		Use:       "sig",
		KeyID:     keyID,
	}}}

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(jwks)
		}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	keys, err := FetchPublicJWKS(u)
	require.NoError(t, err)

	verifyKeys, err := keys.Verify(context.Background())
	require.NoError(t, err)
	require.Len(t, verifyKeys, 1)

	token, err := SignToken(Key{key: privateKey, algo: jose.RS256},
		"1234", "user@example.com", User{Role: "admin"}, tokenExpiresIn)
	require.NoError(t, err)

	claims, err := VerifyToken[User](verifyKeys[0], token)
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", claims.Email)
	assert.Equal(t, "admin", claims.Extra.Role)
}
