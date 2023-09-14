package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
	"github.com/ernestrc/blue/retry"
	log "github.com/sirupsen/logrus"
)

// TokenHTTPHandler returns a http.Handler that handles oauth2 token requests by
// forwarding the request to an upstream channel and intercepting an id token to associate
// with the returned access token. This function returns an error if the given upstreamURL
// could not be parsed as a url.URL.
//
// It uses the given granter to grant extra claims to the user.
func TokenHTTPHandler[T any](
	keys Keys, passwordStore *PasswordStore,
	secretStore SecretStore, granter Granter[T],
	expiry time.Duration,
) http.Handler {
	return tokenHandler[T]{
		secretStore:   secretStore,
		passwordStore: passwordStore,
		signKey:       keys,
		expiry:        expiry,
		granter:       granter,
	}
}

const tokenCallType = "RedeemToken"

type tokenHandler[T any] struct {
	passwordStore *PasswordStore
	secretStore   SecretStore
	signKey       Keys
	granter       Granter[T]
	expiry        time.Duration
}

func (h tokenHandler[T]) ServeHTTP(
	w http.ResponseWriter, in *http.Request,
) {
	traceID, ctx := trace.FromContextOrNew(in.Context())
	logger := log.WithFields(log.Fields{logging.KeyTraceID: traceID})
	attemptAt := logAttempt(tokenCallType, in, traceID)

	body, refreshToken, clientSecret, clientID, err := validateTokenRequest(
		ctx, traceID, attemptAt, w, in)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadRequest,
			response{Message: err.Error()})
		return
	}

	body, redeemURL, tokenURL, certsURL, ok := validateClientSecret(
		ctx, traceID, attemptAt, tokenCallType, w, in, clientSecret, clientID, body,
		h.passwordStore, h.secretStore)
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

	providerResponse, ok := fetchProviderToken(
		ctx, logger, traceID, attemptAt, tokenCallType, w, in, redeemURL, body)
	if !ok {
		return
	}

	claims, ok := validateProviderResponse(
		ctx, logger, traceID, attemptAt, tokenCallType,
		w, in, certsURL, clientID, providerResponse)
	if !ok {
		return
	}

	// override access_token with own token, that we can decode and introspect on middleware
	extra, err := h.granter.Grant(ctx, claims.Subject, claims.Email)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: fmt.Sprintf("grant token: %v", err.Error())})
		return
	}

	writeRedeemTokenResponse(
		ctx, traceID, attemptAt, tokenCallType, w, in, tokenURL, h.signKey,
		h.expiry, providerResponse, claims, extra)
}

func validateSecret(
	ctx context.Context, traceID trace.ID,
	attemptAt time.Time, callType string,
	w http.ResponseWriter, in *http.Request,
	clientSecret, clientID string, passwordStore *PasswordStore,
) (redeemURL, tokenURL, certsURL string, ok bool) {
	metadata, err := passwordStore.VerifyPassword(ctx, clientID, []byte(clientSecret))
	if err != nil {
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusBadRequest,
			response{Message: fmt.Sprintf("verify secret: %v", err.Error())})
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
	tokenURL, _ = metadata[metadataKeyTokenURL]

	return
}

func fetchSecret(
	ctx context.Context, traceID trace.ID, attemptAt time.Time,
	callType string, w http.ResponseWriter, in *http.Request,
	clientID string, secretStore SecretStore,
) (redeemURL, tokenURL, certsURL, clientSecret string, ok bool) {
	// clientIDs are store in base64 encoded, so we don't violate any store key
	// character set constrains.
	clientID = base64.StdEncoding.EncodeToString([]byte(clientID))

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
	tokenURL, _ = metadata[metadataKeyTokenURL]

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
type redeemResponse struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

func validateTokenRequest(
	ctx context.Context, traceID trace.ID, attemptAt time.Time,
	w http.ResponseWriter, in *http.Request,
) (body []byte, refreshToken, clientSecret, clientID string, err error) {
	body, err = ioutil.ReadAll(in.Body)
	if err != nil {
		err = fmt.Errorf("read body: %v", err)
		return
	}

	// reassign the body so we can call ParseForm but use intact data
	// for forwarded request
	in.Body = ioutil.NopCloser(bytes.NewReader(body))
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
	clientSecret = in.PostForm.Get("client_secret")
	refreshToken = in.PostForm.Get("refresh_token")

	// validate by gran type
	switch grantType {
	case "authorization_code":
		challenge := in.PostForm.Get("code_challenge")
		verifier := in.PostForm.Get("verifier")
		if clientID == "" || (clientSecret == "" && (challenge == "" || verifier == "")) {
			err := errors.New("'client_id' must always be set and either " +
				"'client_secret' or pkce params must be set for this grant_type")
			return nil, "", "", "", err
		}
	case "refresh_token":
		if refreshToken == "" {
			err := errors.New("'refresh_token' must always be set for this gran_type")
			return nil, "", "", "", err
		}
	default:
		err := fmt.Errorf("unsupported grant_type: %v", grantType)
		return nil, "", "", "", err
	}

	return
}

// validate that client id and client secret are expected
// and check what provider they're from. To know correct upstream URL.
func validateClientSecret(
	ctx context.Context, traceID trace.ID, attemptAt time.Time,
	callType string, w http.ResponseWriter, in *http.Request,
	clientSecret, clientID string, inBody []byte,
	passwordStore *PasswordStore, secretStore SecretStore,
) (body []byte, redeemURL, tokenURL, certsURL string, ok bool) {
	body = inBody
	if clientSecret != "" {
		redeemURL, tokenURL, certsURL, ok = validateSecret(ctx, traceID, attemptAt, callType,
			w, in, clientSecret, clientID, passwordStore)
	} else {
		redeemURL, tokenURL, certsURL, clientSecret, ok = fetchSecret(ctx, traceID,
			attemptAt, callType, w, in, clientID, secretStore)
		// add client_secret to forwarded request
		if ok {
			in.PostForm.Add("client_secret", clientSecret)
			body = []byte(in.PostForm.Encode())
		}
	}
	return
}

func fetchProviderToken(
	ctx context.Context, logger *log.Entry, traceID trace.ID, attemptAt time.Time,
	callType string, w http.ResponseWriter, in *http.Request,
	redeemURL string, body []byte,
) (ret redeemResponse, ok bool) {
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

		respBody, err = ioutil.ReadAll(res.Body)
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

	if ret.IDToken == "" {
		logger.Warningf("missing ID token in response: possible error response: %s", string(respBody))
		writeResponse(ctx, callType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: "missing ID token in response"})
		return
	}

	ok = true
	return
}

func validateProviderResponse(
	ctx context.Context, logger *log.Entry,
	traceID trace.ID, attemptAt time.Time, callType string,
	w http.ResponseWriter, in *http.Request,
	certsURL, clientID string,
	providerResponse redeemResponse,
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

	logger.Debugf("got clientID %s claims %#v", clientID, claims)

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
	providerResponse redeemResponse, claims *ProviderClaims, extra T,
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

	// if no token URL is available for this provider
	// then do not return a refresh token.
	if tokenURL == "" {
		ret.RefreshToken = ""
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
