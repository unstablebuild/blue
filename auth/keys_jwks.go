// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-jose/go-jose/v4"
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
	defer func() { _ = resp.Body.Close() }()

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
