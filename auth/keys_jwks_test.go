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
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchPublicJWKS(t *testing.T) {
	// NOTE: to make this test work, we should create an http endpoint and return a pub key
	t.SkipNow()

	endpoint := "https://www.googleapis.com/oauth2/v3/certs"
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	keys, err := FetchPublicJWKS(u)
	require.NoError(t, err)

	verifyKeys, err := keys.Verify(context.Background())
	require.NoError(t, err)

	require.True(t, len(verifyKeys) >= 1)

	token := "REDACTED"

	for _, key := range verifyKeys {
		_, err = VerifyToken[any](key, token)
		if err == nil {
			break
		}
	}
	require.NoError(t, err)
}
