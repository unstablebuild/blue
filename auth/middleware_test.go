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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware(t *testing.T) {
	testSignKey, err := SymmetricKey([]byte(symmetricKey))
	require.NoError(t, err)
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
			ctx := context.Background()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				ctx = r.Context()
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
				claims, ok := ClaimsFromContext[User](ctx)
				require.True(t, ok)

				expectedClaims := UserClaims[User]{
					Email:  "1234",
					UserID: "1234",
					Extra: User{
						Role: "Admin",
					},
					Claims: jwt.Claims{
						Issuer:   defaultIssuer,
						Subject:  "1234",
						Audience: defaultAudience,
					},
				}
				claims.Expiry = nil
				claims.IssuedAt = nil
				claims.NotBefore = nil
				claims.ID = ""
				assert.Equal(t, expectedClaims, claims)
			} else {
				assert.Equal(t, http.StatusForbidden, resp.StatusCode)
			}
		})
	}
}
