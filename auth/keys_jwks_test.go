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
