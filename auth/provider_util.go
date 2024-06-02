package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"github.com/unstablebuild/blue/retry"
	"gopkg.in/go-jose/go-jose.v2"
	"gopkg.in/go-jose/go-jose.v2/jwt"
)

var (
	// endpoint URL to redeem a new token.
	metadataKeyRedeemURL = "redeem_url"
	// endpoint URL to refresh tokens with a refresh_token type of flow.
	// This is optional but, if this is not set in the secret metadata,
	// then refresh tokens will be omitted from responses so client will have
	// to do an authorization_code every time, requiring user input.
	metadataKeyTokenURL = "token_url"
	// endpoint URL to pull jwks public keys to validate token signature
	metadataKeyCertsURL = "certs_url"

	httpOutboundRetryStrategy = retry.CombinedStrategy(
		retry.LimitStrategy(10), retry.ExponentialStrategy(100*time.Millisecond, 1*time.Second),
	)
)

// ProviderClaims represents the ID token claims as part
// of an oauth2 flow.
type ProviderClaims struct {
	jwt.Claims
	Email string `json:"email,omitempty"`
}

// ValidateProviderIDWithSecret validates that a given oauth2 ID token is valid
// given a secret provider. The Certs are fetched from the stored certs_url
// in the secret metadata. See ValidateIDWithCerts for more details.
func ValidateProviderIDWithSecret(
	ctx context.Context, store *PasswordStore,
	clientID, clientSecret string, idToken string,
) (*ProviderClaims, error) {
	metadata, err := store.VerifyPassword(ctx, clientID, []byte(clientSecret))
	if err != nil {
		return nil, fmt.Errorf("store verify secret: %v", err)
	}

	certsURL, ok := metadata[metadataKeyCertsURL]
	if !ok {
		return nil, fmt.Errorf("invalid secret: missing %s", metadataKeyCertsURL)
	}

	return ValidateProviderIDWithCertsURL(ctx, certsURL, clientID, idToken)
}

// ValidateProviderIDWithCertsURL fetches a set of certs in JWT format from the given URL
// and uses them to validate the given token ID. See ValidateIDWithCerts for more details.
func ValidateProviderIDWithCertsURL(
	ctx context.Context, certsURL, clientID, idToken string,
) (*ProviderClaims, error) {
	const callType = "ValidateProviderIDWithCertsURL"
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: loggingClass},
		{Key: "ClientID", Value: clientID},
		{Key: "certsURL", Value: certsURL},
	}
	attemptAt := logging.LogAttempt(traceID, callType, fields...)

	var data []byte
	err := retry.Retry(ctx, httpOutboundRetryStrategy, func(ctx context.Context) (bool, error) {
		resp, err := http.Get(certsURL)
		if err != nil {
			return true, fmt.Errorf("get certs url: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return true, fmt.Errorf("status code: %v", resp.StatusCode)
		}

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return true, fmt.Errorf("read certs url body: %v", err)
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	var keys jose.JSONWebKeySet
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("unmarshal certs url body: %v", err)
	}

	if len(keys.Keys) == 0 {
		return nil, errors.New("certs endpoint returned no keys")
	}

	claims, err := ValidateProviderIDWithJWKS(ctx, &keys, clientID, idToken)
	logging.LogResultInfo(err, attemptAt, traceID, callType, fields...)
	return claims, err
}

// ValidateProviderIDWithJWKS verifies that the given id token is valid,
// with the given JWKS.
func ValidateProviderIDWithJWKS(
	ctx context.Context, jwks *jose.JSONWebKeySet, clientID, idToken string,
) (*ProviderClaims, error) {
	// logs "invalid token" requests with traceID for debugging
	traceID, _ := trace.FromContextOrNew(ctx)
	logger := log.WithFields(log.Fields{logging.KeyTraceID: traceID})

	token, err := jwt.ParseSigned(idToken)
	if err != nil {
		logger.Warningf("invalid token: parse: %v", err.Error())
		return nil, nil
	}

	// https://auth0.com/blog/navigating-rs256-and-jwks/
	var signingKey *jose.JSONWebKey
	for _, header := range token.Headers {
		keys := jwks.Key(header.KeyID)
		if len(keys) == 0 {
			continue
		}
		signingKey = &keys[0]
	}

	if signingKey == nil {
		logger.Warningf("could not find suitable key for id token: %v", token.Headers)
		return nil, nil
	}

	var claims ProviderClaims
	if err := token.Claims(signingKey, &claims); err != nil {
		logger.Warningf("invalid token: failed to verify claims: %v", err)
		return nil, nil
	}

	if err := claims.Validate(jwt.Expected{Audience: jwt.Audience{clientID}}); err != nil {
		logger.Warningf("invalid token: invalid client ID claim: %v", err)
		return nil, nil
	}

	if time.Now().After(claims.Expiry.Time()) {
		logger.Warningf("invalid token: token expired on %s", claims.Expiry.Time().String())
		return nil, nil
	}

	return &claims, nil
}
