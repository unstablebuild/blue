package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"gopkg.in/go-jose/go-jose.v2"
)

// FetchPublicJWKS returns a set of Keys that are fetched over http
// as public jwks. This is performed in this method call
// and the results are cached forever or an error is returned.
func FetchPublicJWKS(endpoint *url.URL) (Keys, error) {
	resp, err := http.Get(endpoint.String())
	if err != nil {
		return nil, fmt.Errorf("http get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http get: status code %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var jwks jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("json decode jwk key: %v", err)
	}
	for _, jwk := range jwks.Keys {
		if !jwk.Valid() {
			return nil, errors.New("invalid JWK key")
		}
	}
	return jwksKeys{keys: jwks}, nil
}

type jwksKeys struct {
	keys jose.JSONWebKeySet
}

func (j jwksKeys) Sign(context.Context) (Key, error) {
	for _, k := range j.keys.Keys {
		if !k.IsPublic() {
			return Key{key: k, algo: jose.SignatureAlgorithm(k.Algorithm)}, nil
		}
	}
	return Key{}, errors.New("cannot sign with this set of keys")
}

func (j jwksKeys) Verify(ctx context.Context) (ret []Key, err error) {
	for _, k := range j.keys.Keys {
		ret = append(ret, Key{key: k, algo: jose.SignatureAlgorithm(k.Algorithm)})
	}
	return
}
