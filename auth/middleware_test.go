package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testSignKey = []byte("1234")
	validToken  string
	denyAll     = FuncAuthorizer(func(context.Context, UserClaims, string) error {
		return ErrForbidden
	})
)

func init() {
	var err error
	validToken, err = SignToken(testSignKey, "1234", "1234", RoleAdmin)
	if err != nil {
		panic(err)
	}
}

func TestAuthMiddleware(t *testing.T) {
	suite := []struct {
		description         string
		authorizationHeader string
		authorizer          Authorizer[UserClaims]
		expectCallsNext     bool // or !ExpectForbidden
	}{
		{"rejects missing authorization header", "", AuthorizeAll[UserClaims](), false},
		{"accepts if valid token and authorizer grants", "Bearer " + validToken, AuthorizeAll[UserClaims](), true},
		{"reject if valid token and authorizer does not grant", "Bearer " + validToken, denyAll, false},
		{"reject if invalid token and authorizer does not grant", "", denyAll, false},
	}

	for _, test := range suite {
		test := test
		t.Run(test.description, func(t *testing.T) {
			var called bool
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			sut := WithMiddleware(handler, MiddlewareConfig{SignKey: testSignKey, Authorizer: test.authorizer})

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
