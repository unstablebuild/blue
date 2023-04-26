package auth

import (
	"bytes"
	"context"
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
func TokenHTTPHandler(signKey []byte, store *Store) http.Handler {
	return tokenHandler{store: store, signKey: signKey}
}

const tokenCallType = "RedeemToken"

type tokenHandler struct {
	store   *Store
	signKey []byte
}

func (h tokenHandler) ServeHTTP(
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
	// so client_id and client_secret are expected to be in a POST request's
	// form.
	clientID := in.PostForm.Get("client_id")
	clientSecret := in.PostForm.Get("client_secret")

	// validate that client id and client secret are expected
	// and check what provider they're from. To know correct upstream URL.
	redeemURL, certsURL, ok := h.validateSecret(ctx, traceID, attemptAt, w,
		in, clientID, clientSecret)
	if !ok {
		// validateSecret writes error response
		return
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
	redeem.AccessToken, err = SignToken(h.signKey, claims.Subject, claims.Email, RoleAdmin)
	if err != nil {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusInternalServerError,
			response{Message: fmt.Sprintf("sign token: %v", err.Error())})
		return
	}

	// TODO should support otherwise oauth2 client will have to re-login every time
	// redeem.RefreshToken = ""

	// do not return provider id token to client
	redeem.IDToken = ""

	writeRedeemResponse(ctx, traceID, attemptAt, w, in, redeem)
}

func (h tokenHandler) validateSecret(
	ctx context.Context, traceID trace.ID,
	attemptAt time.Time, w http.ResponseWriter, in *http.Request,
	clientID, clientSecret string,
) (redeemURL, certsURL string, ok bool) {
	if clientID == "" || clientSecret == "" {
		writeResponse(ctx, tokenCallType, traceID, attemptAt, w, in, http.StatusBadRequest,
			response{Message: "missing client ID or client secret in request form"})
		return
	}

	metadata, err := h.store.VerifySecret(ctx, clientID, []byte(clientSecret))
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
