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
