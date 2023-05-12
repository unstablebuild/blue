package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware(t *testing.T) {
	testSignKey := SymmetricKey([]byte("1234"))
	testSignKeys := StaticSymmetricKeys(testSignKey)
	denyAll := FuncAuthorizer(func(context.Context, UserClaims[User], string) error {
		return ErrForbidden
	})
	priv := loadKey(t, "./testdata/jwk-priv.json")
	pub1 := loadKey(t, "./testdata/jwk-pub1.json.pub")
	pub2 := loadKey(t, "./testdata/jwk-pub2.json.pub")
	pub3 := loadKey(t, "./testdata/jwk-pub3.json.pub")
	validToken, err := SignToken(testSignKey, "1234", "1234", User{Role: "Admin"}, 1*time.Hour)
	require.NoError(t, err)
	multiKeys := StaticAsymmetricKeys(priv, pub1, pub2, pub3)

	suite := []struct {
		description         string
		authorizationHeader string
		authorizer          Authorizer[User]
		expectCallsNext     bool // or !ExpectForbidden
		keys                Keys
	}{
		{"rejects missing authorization header", "", AuthorizeAll[User](), false, testSignKeys},
		{"reject if valid token and authorizer does not grant", "Bearer " + validToken, denyAll, false, testSignKeys},
		{"reject if invalid token and authorizer does not grant", "", denyAll, false, testSignKeys},
		{"accepts if exactly one key verifies token and authorizer grants", "Bearer " + validToken, AuthorizeAll[User](), true, testSignKeys},
		{"accepts if at least one key verifies token and authorizer grants", "Bearer " + validToken, AuthorizeAll[User](), true, multiKeys},
	}

	for _, test := range suite {
		test := test
		t.Run(test.description, func(t *testing.T) {
			var called bool
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			sut := WithMiddleware(handler, MiddlewareConfig[User]{VerifyKeys: testSignKeys, Authorizer: test.authorizer})

			req := httptest.NewRequest("GET", "http://localhost:3001/foo", nil)
			if test.authorizationHeader != "" {
				req.Header.Set("Authorization", test.authorizationHeader)
			}

			w := httptest.NewRecorder()
			sut.ServeHTTP(w, req)

			resp := w.Result()

			assert.Equal(t, test.expectCallsNext, called)
			if test.expectCallsNext {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			} else {
				assert.Equal(t, http.StatusForbidden, resp.StatusCode)
			}
		})
	}
}
