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

	"github.com/ernestrc/blue/document"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/go-jose/go-jose.v2"
	"gopkg.in/go-jose/go-jose.v2/jwt"
)

const keyID = "123893721987389217"

var (
	validJWKS     = jose.JSONWebKeySet{}
	rsaPrivateKey *rsa.PrivateKey
	rsaPublicKey  *rsa.PublicKey
	grantAll      = FuncGranter(func(context.Context, string, string) (User, error) {
		return User{Role: "admin"}, nil
	})
)

func init() {
	rsaPrivateKey, _ = rsa.GenerateKey(rand.Reader, 4096)
	rsaPublicKey = &rsaPrivateKey.PublicKey

	validJWKS.Keys = make([]jose.JSONWebKey, 1)
	validJWKS.Keys[0] = jose.JSONWebKey{
		Key:       rsaPublicKey,
		Algorithm: string(jose.RS256),
		Use:       "sig",
		KeyID:     keyID,
	}
}

func TestTokenHandler(t *testing.T) {
	validSecret := []byte("1")
	validClientID := "id: 1"
	invalidSecret := []byte("2")
	invalidClientID := "id: 2"
	secretMissingURLS := []byte("3")
	clientIDMissingURLS := "id: 3"

	suite := []struct {
		description        string
		requestBody        url.Values
		tokenProvider      http.Handler
		certsProvider      http.Handler
		expectedStatusCode int
	}{
		{"missing client_id, client_secret is a bad request",
			url.Values{}, goodRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"unkown client_id is a bad request",
			url.Values{"client_id": []string{invalidClientID}},
			goodRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"empty client_id is a bad request",
			url.Values{"client_id": []string{""}},
			goodRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"empty client_id is a bad request",
			url.Values{"client_id": []string{""}},
			goodRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"client_id with bad client_secret is a bad request",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{string(invalidSecret)}},
			goodRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"client_id with empty client_secret is a bad request",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{""}},
			goodRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusBadRequest},
		{"client_id with valid client_secret but missing urls in auth store is 500",
			url.Values{"client_id": []string{clientIDMissingURLS}, "client_secret": []string{string(secretMissingURLS)}},
			goodRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusInternalServerError},
		{"happy path",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{string(validSecret)}},
			goodRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusOK},
		{"retries redeem endpoints 5xx",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{string(validSecret)}},
			flakyRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusOK},
		{"retries certs endpoint 5xx",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{string(validSecret)}},
			goodRedeemHandler(validClientID), flakyCertsHandler(&validJWKS), http.StatusOK},
		{"malformed redeem endpoint response is 502",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{string(validSecret)}},
			malformedHandler(), goodCertsHandler(&validJWKS), http.StatusBadGateway},
		{"malformed certs endpoint response is 502",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{string(validSecret)}},
			goodRedeemHandler(validClientID), malformedHandler(), http.StatusBadGateway},
		{"missing keys from certs endpoint response is 502",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{string(validSecret)}},
			goodRedeemHandler(validClientID), goodCertsHandler(&jose.JSONWebKeySet{}), http.StatusBadGateway},
		{"redeem provider token audience does not match clientID is 424",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{string(validSecret)}},
			goodRedeemHandler(invalidClientID), goodCertsHandler(&validJWKS), http.StatusFailedDependency},
		{"redeem provider returned expired token is 424",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{string(validSecret)}},
			expiredRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusFailedDependency},
		{"redeem provider returned token with non-matching signature is 424",
			url.Values{"client_id": []string{validClientID}, "client_secret": []string{string(validSecret)}},
			badRedeemHandler(validClientID), goodCertsHandler(&validJWKS), http.StatusFailedDependency},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			tokenServer := httptest.NewServer(test.tokenProvider)
			defer tokenServer.Close()

			certsServer := httptest.NewServer(test.certsProvider)
			defer certsServer.Close()

			store := NewStore(document.NewInMemoryService())
			store.CreateSecret(context.Background(), validClientID, validSecret, map[string]string{
				metadataKeyRedeemURL: tokenServer.URL,
				metadataKeyCertsURL:  certsServer.URL,
			})
			store.CreateSecret(context.Background(), clientIDMissingURLS, secretMissingURLS, nil)

			req := httptest.NewRequest("POST", "http://localhost:3001/o/oauth2/token",
				strings.NewReader(test.requestBody.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()
			sut := TokenHTTPHandler(testSignKey, store, grantAll)
			sut.ServeHTTP(w, req)

			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)

			require.Equal(t, test.expectedStatusCode, resp.StatusCode)
			if test.expectedStatusCode != http.StatusOK {
				return
			}

			var actualOut redeemResponse
			err := json.Unmarshal(body, &actualOut)
			require.NoError(t, err)

			_, err = VerifyToken[User](testSignKey, actualOut.AccessToken)
			require.NoError(t, err)

			assert.NotZero(t, actualOut.ExpiresIn)
			assert.NotZero(t, actualOut.Scope)
			assert.NotZero(t, actualOut.TokenType)
			assert.Zero(t, actualOut.IDToken)
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
	raw, err := jwt.Signed(sig).Claims(claims).CompactSerialize()
	if err != nil {
		logrus.Errorf("signer sign: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var out redeemResponse
	out.IDToken = raw
	out.ExpiresIn = int(time.Until(time.Now().Add(expiresIn)).Seconds())
	out.Scope = "blabla"
	out.TokenType = "something"

	data, err := json.Marshal(out)
	if err != nil {
		logrus.Errorf("signer sign: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func goodRedeemHandler(clientID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestRedeemResponse(w, clientID, 24*time.Hour, rsaPrivateKey)
	})
}

func badRedeemHandler(clientID string) http.Handler {
	otherKey, _ := rsa.GenerateKey(rand.Reader, 4096)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestRedeemResponse(w, clientID, 24*time.Hour, otherKey)
	})
}

func expiredRedeemHandler(clientID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestRedeemResponse(w, clientID, -1*time.Hour, rsaPrivateKey)
	})
}

func flakyRedeemHandler(clientID string) http.Handler {
	var attempt int
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeTestRedeemResponse(w, clientID, 24*time.Hour, rsaPrivateKey)
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
