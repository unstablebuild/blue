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
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	keyID             = "123893721987389217"
	providerExpiresIn = 24 * time.Hour
	tokenExpiresIn    = 1 * time.Hour
)

func TestTokenHandler(t *testing.T) {
	grantAll := FuncGranter(func(context.Context, *ProviderClaims) (User, error) {
		return User{Role: "admin"}, nil
	})
	testSignKey, err := SymmetricKey([]byte(symmetricKey))
	require.NoError(t, err)
	testSignKeys := StaticSymmetricKeys(testSignKey)
	rsaPrivateKey, _ := rsa.GenerateKey(rand.Reader, 4096)
	rsaPublicKey := &rsaPrivateKey.PublicKey

	var validJWKS jose.JSONWebKeySet
	validJWKS.Keys = make([]jose.JSONWebKey, 1)
	validJWKS.Keys[0] = jose.JSONWebKey{
		Key:       rsaPublicKey,
		Algorithm: string(jose.RS256),
		Use:       "sig",
		KeyID:     keyID,
	}

	validClientID := "id: 1"
	invalidClientID := "id: 2"
	clientIDMissingURLS := "id: 3"
	clientIDMissingTokenURLS := "id: 4"

	suite := []struct {
		description        string
		requestBody        url.Values
		tokenProvider      http.Handler
		certsProvider      http.Handler
		expectedStatusCode int
	}{
		{"missing client_id is a bad request",
			url.Values{"grant_type": []string{"authorization_code"}}, goodRedeemHandler(validClientID, rsaPrivateKey),
			goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"unkown client_id is a bad request",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{invalidClientID}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"empty client_id is a bad request",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{""}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"empty client_id is a bad request",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{""}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"client_id with empty pkce code_challenge is a bad request",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{""}, "code_verifier": []string{"1234"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"client_id with empty pkce code_verifier is a bad request",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{""}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"client_id with valid PKCE params but missing urls in auth store is 500",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{clientIDMissingURLS},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusInternalServerError},
		{"missing grant_type is a 400",
			url.Values{"client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"incorrect grant_type with PKCE",
			url.Values{"grant_type": []string{"AHORA!"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"refresh token grant type without refresh_token field is 400",
			url.Values{"grant_type": []string{"refresh_token"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"happy path with PKCE",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusOK},
		{"refresh token, with PKCE, with missing clientID is a 400",
			url.Values{"grant_type": []string{"refresh_token"}, "client_id": []string{},
				"refresh_token": []string{"123455666"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"happy path refresh token with PKCE",
			url.Values{"grant_type": []string{"refresh_token"}, "client_id": []string{validClientID},
				"refresh_token": []string{"123455666"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusOK},
		{"refresh token flow with secret missing token url is 500",
			url.Values{"grant_type": []string{"refresh_token"}, "client_id": []string{clientIDMissingTokenURLS},
				"refresh_token": []string{"123455666"}},
			goodRedeemHandler(clientIDMissingTokenURLS, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusInternalServerError},
		{"PKCE flow with missing secret in store is 500",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{invalidClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusInternalServerError},
		{"retries redeem endpoints 5xx",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			flakyRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusOK},
		{"retries certs endpoint 5xx",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), flakyCertsHandler(&validJWKS), http.StatusOK},
		{"malformed redeem endpoint response is 502",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			malformedHandler(), goodCertsHandler(&validJWKS), http.StatusBadGateway},
		{"malformed certs endpoint response is 502",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), malformedHandler(), http.StatusBadGateway},
		{"missing keys from certs endpoint response is 502",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			goodRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&jose.JSONWebKeySet{}), http.StatusBadGateway},
		{"redeem provider token audience does not match clientID is 424",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			goodRedeemHandler(invalidClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusFailedDependency},
		{"redeem provider returned expired token is 424",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			expiredRedeemHandler(validClientID, rsaPrivateKey), goodCertsHandler(&validJWKS), http.StatusFailedDependency},
		{"redeem provider returned token with non-matching signature is 424",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			badRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusFailedDependency},
		{"provider returned other json response is 400",
			url.Values{"grant_type": []string{"authorization_code"}, "client_id": []string{validClientID},
				"code_challenge": []string{"1234"}, "code_verifier": []string{"1234"}, "code_challenge_method": []string{"plain"}},
			goodRedeemHandlerBadResponse(validClientID), goodCertsHandler(&validJWKS), http.StatusBadRequest},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			tokenServer := httptest.NewServer(test.tokenProvider)
			defer tokenServer.Close()

			certsServer := httptest.NewServer(test.certsProvider)
			defer certsServer.Close()

			encodedClientID := EncodeSecretID(validClientID)
			secretStore := MapSecretStore(map[string][]byte{encodedClientID: []byte("1")},
				metadataKeyRedeemURL, tokenServer.URL,
				metadataKeyTokenURL, tokenServer.URL,
				metadataKeyCertsURL, certsServer.URL,
			)

			req := httptest.NewRequest("POST", "http://localhost:3001/o/oauth2/token",
				strings.NewReader(test.requestBody.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()
			sut := TokenHTTPHandler(testSignKeys, secretStore, grantAll, tokenExpiresIn)
			sut.ServeHTTP(w, req)

			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)

			require.Equal(t, test.expectedStatusCode, resp.StatusCode)
			if test.expectedStatusCode != http.StatusOK {
				return
			}

			var actualOut redeemResponse[User]
			err := json.Unmarshal(body, &actualOut)
			require.NoError(t, err)

			_, err = VerifyToken[User](testSignKey, actualOut.AccessToken)
			require.NoError(t, err)

			assert.Equal(t, int(tokenExpiresIn.Seconds()), actualOut.ExpiresIn)
			assert.NotZero(t, actualOut.Scope)
			assert.NotZero(t, actualOut.TokenType)
			assert.Zero(t, actualOut.IDToken)
			assert.Equal(t, "admin", actualOut.Extra.Role)
		})
	}
}

func writeTestRedeemResponse(
	w http.ResponseWriter, clientID string,
	expiresIn time.Duration, privateKey *rsa.PrivateKey,
) {
	claims := ProviderClaims{
		Email: "it@unstable.build",
		Claims: jwt.Claims{
			Expiry:    jwt.NewNumericDate(time.Now().Add(expiresIn)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    defaultIssuer,
			Subject:   "1234",
			ID:        uuid.New().String(),
			Audience:  jwt.Audience{clientID},
		},
	}

	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", keyID))
	if err != nil {
		logrus.Errorf("create signer: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	raw, err := jwt.Signed(sig).Claims(claims).Serialize()
	if err != nil {
		logrus.Errorf("signer sign: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var out redeemResponse[User]
	out.IDToken = raw
	out.ExpiresIn = int(expiresIn.Seconds())
	out.Scope = "blabla"
	out.TokenType = "something"

	data, err := json.Marshal(out)
	if err != nil {
		logrus.Errorf("signer sign: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func goodRedeemHandler(clientID string, rsaPrivateKey *rsa.PrivateKey) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestRedeemResponse(w, clientID, providerExpiresIn, rsaPrivateKey)
	})
}

func badRedeemHandler(clientID string) http.Handler {
	otherKey, _ := rsa.GenerateKey(rand.Reader, 4096)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestRedeemResponse(w, clientID, providerExpiresIn, otherKey)
	})
}

func goodRedeemHandlerBadResponse(clientID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var out redeemResponse[User]
		out.TokenType = "somethingWeird"

		data, err := json.Marshal(out)
		if err != nil {
			logrus.Errorf("signer sign: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})
}

func expiredRedeemHandler(clientID string, rsaPrivateKey *rsa.PrivateKey) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestRedeemResponse(w, clientID, -1*time.Hour, rsaPrivateKey)
	})
}

func flakyRedeemHandler(clientID string, rsaPrivateKey *rsa.PrivateKey) http.Handler {
	var attempt int
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeTestRedeemResponse(w, clientID, providerExpiresIn, rsaPrivateKey)
	})
}

func writeTestCertsResponse(w http.ResponseWriter, jwks *jose.JSONWebKeySet) {
	data, err := json.Marshal(jwks)
	if err != nil {
		logrus.Errorf("marshal jwks: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(data); err != nil {
		logrus.Errorf("http write: %v", err)
		return
	}
}

func goodCertsHandler(jwks *jose.JSONWebKeySet) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestCertsResponse(w, jwks)
	})
}

func malformedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("\x00jfkeljfklw")); err != nil {
			logrus.Errorf("http write: %v", err)
			return
		}
	})
}

func flakyCertsHandler(jwks *jose.JSONWebKeySet) http.Handler {
	var attempt int
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeTestCertsResponse(w, jwks)
	})
}
