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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"github.com/unstablebuild/blue/retry"
)

// TokenHTTPHandler returns a http.Handler that handles oauth2 token requests by
// forwarding the request to an upstream channel and intercepting an id token to associate
// with the returned access token. This function returns an error if the given upstreamURL
// could not be parsed as a url.URL.
//
// It uses the given granter to grant extra claims to the user.
//
// If client secret is not present in the request (i.e. PCKE)
// then a client secret is fetched from secretStore with the client id
// as the base64 encoded (url encoded, no padding) as the secret id.
func TokenHTTPHandler[T any](
	keys Keys, secretStore SecretStore, granter Granter[T],
	expiry time.Duration,
) http.Handler {
	return tokenHandler[T]{
		secretStore: secretStore,
		signKey:     keys,
		expiry:      expiry,
		granter:     granter,
	}
}

const tokenCallType = "Oauth2RedeemToken"

type tokenHandler[T any] struct {
	secretStore SecretStore
	signKey     Keys
	granter     Granter[T]
	expiry      time.Duration
}

func (h tokenHandler[T]) ServeHTTP(
	w http.ResponseWriter, in *http.Request,
) {
	traceID, ctx := trace.FromContextOrNew(in.Context())
	logger := log.WithFields(log.Fields{logging.KeyTraceID: traceID})
	attemptAt := logAttempt(tokenCallType, in, traceID)

	body, refreshToken, clientID, err := validateTokenRequest(
		ctx, traceID, attemptAt, w, in)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadRequest,
			response{Message: err.Error()})
		return
	}

	body, redeemURL, tokenURL, certsURL, ok := validateClientSecret(
		ctx, traceID, attemptAt, tokenCallType, w, in, clientID, body,
		h.secretStore)
	if !ok {
		// validateClientSecret writes response
		return
	}

	// use token URL to refresh tokens
	if refreshToken != "" {
		if tokenURL == "" {
			writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
				response{Message: "secret does not have a token URL, but token refresh attempted"})
			return
		}
		redeemURL = tokenURL
	}

	providerResponse, ok := fetchProviderToken[T](
		ctx, logger, traceID, attemptAt, tokenCallType, w, in, redeemURL, body)
	if !ok {
		return
	}

	claims, ok := validateProviderResponse[T](
		ctx, logger, traceID, attemptAt, tokenCallType,
		w, in, certsURL, clientID, providerResponse)
	if !ok {
		return
	}

	// override access_token with own token, that we can decode and introspect on middleware
	extra, err := h.granter.Grant(ctx, claims)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, document.ErrNotFound) {
			status = http.StatusExpectationFailed
		}
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, status,
			response{Message: fmt.Sprintf("grant token: %v", err.Error())})
		return
	}

	writeRedeemTokenResponse(
		ctx, traceID, attemptAt, tokenCallType, w, in, tokenURL, h.signKey,
		h.expiry, providerResponse, claims, extra)
}

func fetchSecret(
	ctx context.Context, traceID trace.ID, attemptAt time.Time,
	callType string, w http.ResponseWriter, in *http.Request,
	clientID string, secretStore SecretStore,
) (redeemURL, tokenURL, certsURL, clientSecret string, ok bool) {
	// clientIDs are store in base64 encoded, so we don't violate any store key
	// character set constrains. Use no padding as secretmanager doesn't
	// permit '=' characters.
	clientID = base64.StdEncoding.WithPadding(base64.NoPadding).
		EncodeToString([]byte(clientID))

	metadata, err := secretStore.GetSecretMetadata(ctx, clientID)
	if err != nil {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusBadRequest,
			response{Message: fmt.Sprintf("get secret metadata: %v", err)})
		return
	}

	if certsURL, ok = metadata[metadataKeyCertsURL]; !ok {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: "secret does not have a keys URL"})
		return
	}

	redeemURL, ok = metadata[metadataKeyRedeemURL]
	if !ok {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: "secret does not have a redeem URL"})
		return
	}

	// optional
	tokenURL = metadata[metadataKeyTokenURL]

	clientSecretData, err := secretStore.AccessSecret(ctx, clientID)
	if err != nil {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: fmt.Sprintf("access secret data: %v", err)})
		return
	}
	clientSecret = string(clientSecretData)

	return
}

/*
	 {
	   "access_token": "",
	  "expires_in": 3599,
	  "refresh_token": "",
	  "scope": "https://www.googleapis.com/auth/userinfo.email openid",
	  "token_type": "Bearer",
	  "id_token": "",
	}
*/
type redeemResponse[T any] struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	Extra        T      `json:"extra"`
}

func validateTokenRequest(
	ctx context.Context, traceID trace.ID, attemptAt time.Time,
	w http.ResponseWriter, in *http.Request,
) (body []byte, refreshToken, clientID string, err error) {
	body, err = io.ReadAll(in.Body)
	if err != nil {
		err = fmt.Errorf("read body: %v", err)
		return
	}

	// reassign the body so we can call ParseForm but use intact data
	// for forwarded request
	in.Body = io.NopCloser(bytes.NewReader(body))
	err = in.ParseForm()
	if err != nil {
		err = fmt.Errorf("parse request form: %v", err)
		return
	}

	// NOTE: we do not support RFC 6749 section 2.3.1 (on basic auth),
	// so all oauth2 params are expected to be in a POST request's
	// form.
	grantType := in.PostForm.Get("grant_type")
	clientID = in.PostForm.Get("client_id")
	refreshToken = in.PostForm.Get("refresh_token")

	// validate by gran type
	switch grantType {
	case "authorization_code":
		challenge := in.PostForm.Get("code_challenge")
		verifier := in.PostForm.Get("code_verifier")
		if clientID == "" || challenge == "" || verifier == "" {
			err := errors.New("'client_id' and pkce params must always be set for this grant_type")
			return nil, "", "", err
		}
	case "refresh_token":
		if refreshToken == "" || clientID == "" {
			err := errors.New("'refresh_token' and 'client_id' must always be set for this gran_type")
			return nil, "", "", err
		}
	default:
		err := fmt.Errorf("unsupported grant_type: %v", grantType)
		return nil, "", "", err
	}

	return
}

// validate that client id and client secret are expected
// and check what provider they're from. To know correct upstream URL.
func validateClientSecret(
	ctx context.Context, traceID trace.ID, attemptAt time.Time,
	callType string, w http.ResponseWriter, in *http.Request,
	clientID string, inBody []byte,
	secretStore SecretStore,
) (body []byte, redeemURL, tokenURL, certsURL string, ok bool) {
	body = inBody

	var clientSecret string
	redeemURL, tokenURL, certsURL, clientSecret, ok = fetchSecret(ctx, traceID,
		attemptAt, callType, w, in, clientID, secretStore)
	// add client_secret to forwarded request
	if ok {
		in.PostForm.Add("client_secret", clientSecret)
		body = []byte(in.PostForm.Encode())
	}
	return
}

func fetchProviderToken[T any](
	ctx context.Context, logger *log.Entry, traceID trace.ID, attemptAt time.Time,
	callType string, w http.ResponseWriter, in *http.Request,
	redeemURL string, body []byte,
) (ret redeemResponse[T], ok bool) {
	// clone incoming request
	out, err := http.NewRequestWithContext(ctx, in.Method, redeemURL, bytes.NewReader(body))
	if err != nil {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: fmt.Sprintf("create outbound redeem request: %v", err.Error())})
		return
	}
	out.Header = make(http.Header)
	for h, val := range in.Header {
		if h != "Accept-Encoding" {
			out.Header[h] = val
		}
	}

	var respBody []byte
	err = retry.Retry(ctx, httpOutboundRetryStrategy, func(ctx context.Context) (bool, error) {
		res, err := http.DefaultClient.Do(out)
		if err != nil {
			return true, err
		}
		if res.StatusCode >= 500 {
			return true, fmt.Errorf("response code %v", res.StatusCode)
		}

		defer res.Body.Close()

		respBody, err = io.ReadAll(res.Body)
		if err != nil {
			return true, fmt.Errorf("read response body: %v", err)
		}
		return false, nil
	})
	if err != nil {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusBadGateway,
			response{Message: fmt.Sprintf("forward request to upstream: %v", err.Error())})
		return
	}

	if err := json.Unmarshal(respBody, &ret); err != nil {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusBadGateway,
			response{Message: fmt.Sprintf("failed to unmarshal upstream response: %v", err.Error())})
		return
	}

	ok = true
	return
}

func validateProviderResponse[T any](
	ctx context.Context, logger *log.Entry,
	traceID trace.ID, attemptAt time.Time, callType string,
	w http.ResponseWriter, in *http.Request,
	certsURL, clientID string,
	providerResponse redeemResponse[T],
) (*ProviderClaims, bool) {
	claims, err := ValidateProviderIDWithCertsURL(ctx, certsURL, clientID, providerResponse.IDToken)
	if err != nil {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusBadGateway,
			response{Message: err.Error()})
		return nil, false
	}
	if claims == nil {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusFailedDependency,
			response{Message: "invalid token"})
		return nil, false
	}

	if claims.Subject == "" {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusBadGateway,
			response{Message: "invalid 'sub' from upstream provider claims"})
		return nil, false
	}
	return claims, true
}

func writeRedeemTokenResponse[T any](
	ctx context.Context, traceID trace.ID, attemptAt time.Time, callType string,
	w http.ResponseWriter, in *http.Request,
	tokenURL string, signKey Keys, expiry time.Duration,
	providerResponse redeemResponse[T], claims *ProviderClaims, extra T,
) {
	ret := providerResponse

	key, err := signKey.Sign(ctx)
	if err != nil {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: fmt.Sprintf("get sign key: %v", err.Error())})
		return
	}

	// override ExpiresIn in response to match what we use to sign the token, which
	// is probably different than what the oauth2 provider has configured.
	ret.ExpiresIn = int(expiry.Seconds())

	ret.AccessToken, err = SignToken(key, claims.Subject, claims.Email, extra, expiry)
	if err != nil {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: fmt.Sprintf("sign token: %v", err.Error())})
		return
	}

	// do not return provider id token to client
	ret.IDToken = ""
	ret.Extra = extra

	// if no token URL is available for this provider
	// then do not return a refresh token.
	if tokenURL == "" {
		ret.RefreshToken = ""
		log.Warnf("removing 'refresh_token' from response: "+
			"client secret does not have a '%s' set in metadata",
			metadataKeyTokenURL)
	}

	fields := []logging.Field{
		{Key: logging.KeyClass, Value: loggingClass},
		{Key: "Method", Value: in.Method},
		{Key: "URL", Value: in.URL.String()},
		{Key: "Status", Value: strconv.Itoa(http.StatusOK)},
		{Key: "ResponseExpiresIn", Value: strconv.Itoa(ret.ExpiresIn)},
		{Key: "ResponseScope", Value: ret.Scope},
		{Key: "ResponseTokenType", Value: ret.TokenType},
	}
	data, err := json.Marshal(ret)
	if err != nil {
		logging.LogResultInfo(err, attemptAt, traceID, callType, fields...)
		return
	}

	fields = append(fields, logging.Field{Key: "ResponseLen", Value: strconv.Itoa(len(data))})

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(data)
	logging.LogResultInfo(err, attemptAt, traceID, callType, fields...)
}
