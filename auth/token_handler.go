package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
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
// It uses the given granter to
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

	body, err := ioutil.ReadAll(in.Body)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadRequest,
			response{Message: fmt.Sprintf("read body: %v", err.Error())})
		return
	}

	// reassign the body so we can call ParseForm but use intact data
	// for forwarded request
	in.Body = ioutil.NopCloser(bytes.NewReader(body))
	if err := in.ParseForm(); err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadRequest,
			response{Message: fmt.Sprintf("parse request form: %v", err.Error())})
		return
	}

	// NOTE: we do not support RFC 6749 section 2.3.1 (on basic auth),
	// so all oauth2 params are expected to be in a POST request's
	// form.
	grantType := in.PostForm.Get("grant_type")
	clientID := in.PostForm.Get("client_id")
	clientSecret := in.PostForm.Get("client_secret")
	refreshToken := in.PostForm.Get("refresh_token")

	// validate by gran type
	switch grantType {
	case "authorization_code":
		challenge := in.PostForm.Get("code_challenge")
		verifier := in.PostForm.Get("verifier")
		if clientID == "" || (clientSecret == "" && (challenge == "" || verifier == "")) {
			writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadRequest,
				response{Message: "'client_id' must always be set and either " +
					"'client_secret' or pkce params must be set for this grant_type"})
			return
		}
	case "refresh_token":
		if refreshToken == "" {
			writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadRequest,
				response{Message: "'refresh_token' must always be set for this gran_type"})
			return
		}
	default:
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadRequest,
			response{Message: fmt.Sprintf("unsupported grant_type: %v", grantType)})
		return
	}

	// validate that client id and client secret are expected
	// and check what provider they're from. To know correct upstream URL.
	var redeemURL, tokenURL, certsURL string
	var ok bool
	if clientSecret != "" {
		redeemURL, tokenURL, certsURL, ok = h.validateSecret(ctx, traceID, attemptAt, w,
			in, clientID, clientSecret)
	} else {
		redeemURL, tokenURL, certsURL, clientSecret, ok = h.fetchSecret(ctx, traceID,
			attemptAt, w, in, clientID)
		// add client_secret to forwarded request
		if ok {
			in.PostForm.Add("client_secret", clientSecret)
			body = []byte(in.PostForm.Encode())
		}

	}
	if !ok {
		// validateSecret/fetchSecret writes error response
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

	// clone incoming request
	out, err := http.NewRequestWithContext(ctx, in.Method, redeemURL, bytes.NewReader(body))
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
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
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadGateway,
			response{Message: fmt.Sprintf("forward request to upstream: %v", err.Error())})
		return
	}

	var redeem redeemResponse
	if err := json.Unmarshal(respBody, &redeem); err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadGateway,
			response{Message: fmt.Sprintf("failed to unmarshal upstream response: %v", err.Error())})
		return
	}

	if redeem.IDToken == "" {
		logger.Warningf("missing ID token in response: possible error response: %s", string(respBody))
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: "missing ID token in response"})
		return
	}

	claims, err := ValidateProviderIDWithCertsURL(ctx, certsURL, clientID, redeem.IDToken)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadGateway,
			response{Message: err.Error()})
		return
	}
	if claims == nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusFailedDependency,
			response{Message: "invalid token"})
		return
	}

	logger.Debugf("got clientID %s claims %#v", clientID, claims)

	if claims.Subject == "" {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadGateway,
			response{Message: "invalid 'sub' from upstream provider claims"})
		return
	}

	// override access_token with own token, that we can decode and introspect on middleware
	extra, err := h.granter.Grant(ctx, claims.Subject, claims.Email)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: fmt.Sprintf("grant token: %v", err.Error())})
		return
	}

	key, err := h.signKey.Sign(ctx)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: fmt.Sprintf("get sign key: %v", err.Error())})
		return
	}

	// override ExpiresIn in response to match what we use to sign the token, which
	// is probably different than what the oauth2 provider has configured.
	redeem.ExpiresIn = int(h.expiry.Seconds())

	redeem.AccessToken, err = SignToken(key, claims.Subject, claims.Email, extra, h.expiry)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: fmt.Sprintf("sign token: %v", err.Error())})
		return
	}

	// do not return provider id token to client
	redeem.IDToken = ""

	// if no token URL is available for this provider
	// then do not return a refresh token.
	if tokenURL == "" {
		redeem.RefreshToken = ""
	}

	writeRedeemResponse(ctx, traceID, attemptAt, w, in, redeem)
}

func (h tokenHandler[T]) validateSecret(
	ctx context.Context, traceID trace.ID,
	attemptAt time.Time, w http.ResponseWriter, in *http.Request,
	clientID, clientSecret string,
) (redeemURL, tokenURL, certsURL string, ok bool) {
	metadata, err := h.passwordStore.VerifyPassword(ctx, clientID, []byte(clientSecret))
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadRequest,
			response{Message: fmt.Sprintf("verify secret: %v", err.Error())})
		return
	}
	if certsURL, ok = metadata[metadataKeyCertsURL]; !ok {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: "secret does not have a keys URL"})
		return
	}

	redeemURL, ok = metadata[metadataKeyRedeemURL]
	if !ok {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: "secret does not have a redeem URL"})
		return
	}
	tokenURL, _ = metadata[metadataKeyTokenURL]

	return
}

func (h tokenHandler[T]) fetchSecret(
	ctx context.Context, traceID trace.ID,
	attemptAt time.Time, w http.ResponseWriter, in *http.Request,
	clientID string,
) (redeemURL, tokenURL, certsURL, clientSecret string, ok bool) {
	// clientIDs are store in base64 encoded, so we don't violate any store key
	// character set constrains.
	clientID = base64.StdEncoding.EncodeToString([]byte(clientID))

	metadata, err := h.secretStore.GetSecretMetadata(ctx, clientID)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadRequest,
			response{Message: fmt.Sprintf("get secret metadata: %v", err)})
		return
	}

	if certsURL, ok = metadata[metadataKeyCertsURL]; !ok {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: "secret does not have a keys URL"})
		return
	}

	redeemURL, ok = metadata[metadataKeyRedeemURL]
	if !ok {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: "secret does not have a redeem URL"})
		return
	}

	// optional
	tokenURL, _ = metadata[metadataKeyTokenURL]

	clientSecretData, err := h.secretStore.AccessSecret(ctx, clientID)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
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

func writeRedeemResponse(
	ctx context.Context, traceID trace.ID,
	attemptAt time.Time, w http.ResponseWriter,
	req *http.Request, r redeemResponse,
) {
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: loggingClass},
		{Key: "Method", Value: req.Method},
		{Key: "URL", Value: req.URL.String()},
		{Key: "Status", Value: strconv.Itoa(http.StatusOK)},
		{Key: "ResponseExpiresIn", Value: strconv.Itoa(r.ExpiresIn)},
		{Key: "ResponseScope", Value: r.Scope},
		{Key: "ResponseTokenType", Value: r.TokenType},
	}
	data, err := json.Marshal(r)
	if err != nil {
		logging.LogResultInfo(err, attemptAt, traceID, tokenCallType, fields...)
		return
	}

	fields = append(fields, logging.Field{Key: "ResponseLen", Value: strconv.Itoa(len(data))})

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(data)
	logging.LogResultInfo(err, attemptAt, traceID, tokenCallType, fields...)
}
