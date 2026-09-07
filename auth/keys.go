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
	"fmt"

	//nolint:all
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"github.com/go-jose/go-jose/v4"
)

// Key is one of the signing key types used by go-jose.
type Key struct {
	key  any
	algo jose.SignatureAlgorithm
}

// Keys abstracts the ability to fetch signing/verification keys.
type Keys interface {
	Sign(context.Context) (Key, error)
	Verify(context.Context) ([]Key, error)
}

// GenerateKeys generates cryptographic key pair, and packages it as a set of Keys.
func GenerateKeys() (Keys, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate rsa key: %w", err)
	}
	return StaticAsymmetricKeys(
		Key{key: privKey, algo: getKeyAlgo(privKey)},
		Key{key: pubKey, algo: getKeyAlgo(pubKey)},
	), nil
}

// StaticSymmetricKeys returns an implementation of Keys that returns
// the given key in calls to Sign and Verify.
func StaticSymmetricKeys(key Key) Keys {
	return staticSymmetricKeys{key: key}
}

// StaticAsymmetricKeys returns an implementation of Keys that returns
// the given key pair in calls to Sign and Verify.
func StaticAsymmetricKeys(priv Key, pub ...Key) Keys {
	return staticAsymmetricKeys{pub: pub, priv: priv}
}

// SymmetricKey returns a symmetric Key with str set as the key.
func SymmetricKey(data []byte) (Key, error) {
	// See jose.SigningKey for more details.
	if len(data) < 32 {
		return Key{}, fmt.Errorf("key must at least have 32 bytes")
	}
	return Key{key: data, algo: jose.HS256}, nil
}

// LoadPublicKey loads a public key from PEM/DER/JWK-encoded data.
func LoadPublicKey(data []byte) (Key, error) {
	input := data

	block, _ := pem.Decode(data)
	if block != nil {
		input = block.Bytes
	}

	// Try to load SubjectPublicKeyInfo
	pub, err0 := x509.ParsePKIXPublicKey(input)
	if err0 == nil {
		return Key{key: pub, algo: getKeyAlgo(pub)}, nil
	}

	cert, err1 := x509.ParseCertificate(input)
	if err1 == nil {
		algo, err := certAlgo(cert.SignatureAlgorithm)
		if err != nil {
			return Key{}, err
		}
		return Key{key: cert.PublicKey, algo: algo}, nil
	}

	jwk, err2 := loadJSONWebKey(data, true)
	if err2 == nil {
		return Key{key: jwk, algo: jose.SignatureAlgorithm(jwk.Algorithm)}, nil
	}

	return Key{}, errors.New("parse error, invalid public key")
}

// LoadPrivateKey loads a private key from PEM/DER/JWK-encoded data.
func LoadPrivateKey(data []byte) (Key, error) {
	input := data

	block, _ := pem.Decode(data)
	if block != nil {
		input = block.Bytes
	}

	var priv interface{}
	priv, err0 := x509.ParsePKCS1PrivateKey(input)
	if err0 == nil {
		return Key{key: priv, algo: getKeyAlgo(priv)}, nil
	}

	priv, err1 := x509.ParsePKCS8PrivateKey(input)
	if err1 == nil {
		return Key{key: priv, algo: getKeyAlgo(priv)}, nil
	}

	priv, err2 := x509.ParseECPrivateKey(input)
	if err2 == nil {
		return Key{key: priv, algo: getKeyAlgo(priv)}, nil
	}

	jwk, err3 := loadJSONWebKey(input, false)
	if err3 == nil {
		return Key{key: jwk, algo: jose.SignatureAlgorithm(jwk.Algorithm)}, nil
	}

	return Key{}, errors.New("parse error, invalid private key")
}

func loadJSONWebKey(json []byte, pub bool) (*jose.JSONWebKey, error) {
	var jwk jose.JSONWebKey
	err := jwk.UnmarshalJSON(json)
	if err != nil {
		return nil, err
	}
	if !jwk.Valid() {
		return nil, errors.New("invalid JWK key")
	}
	if jwk.IsPublic() != pub {
		return nil, errors.New("priv/pub JWK key mismatch")
	}
	return &jwk, nil
}

func getKeyAlgo(key interface{}) jose.SignatureAlgorithm {
	switch k := key.(type) {
	case *rsa.PrivateKey, *rsa.PublicKey:
		// one of RS256, RS384, RS512, PS256, PS384, PS512
		return jose.RS256
	case *ecdsa.PrivateKey:
		curveBits := k.Curve.Params().BitSize
		switch curveBits {
		case 256:
			return jose.ES256
		case 384:
			return jose.ES384
		case 512:
			return jose.ES512
		default:
			panic("invalid curve bitsize for ecdsa private key")
		}
	case *ecdsa.PublicKey,
		*dsa.PublicKey, *dsa.PrivateKey:
		// one of ES256, ES384, ES512:
		return jose.ES256
	case ed25519.PrivateKey, ed25519.PublicKey:
		return jose.EdDSA
	default: // assume symmetric
		return jose.HS256
	}
}

var validAlgorithms = []jose.SignatureAlgorithm{
	jose.RS256,
	jose.RS384,
	jose.RS512,
	jose.ES256,
	jose.ES384,
	jose.ES512,
	jose.PS256,
	jose.PS384,
	jose.PS512,
	jose.EdDSA,
	jose.HS256,
}

func certAlgo(algo x509.SignatureAlgorithm) (jose.SignatureAlgorithm, error) {
	switch algo {

	case x509.SHA256WithRSA:
		return jose.RS256, nil
	case x509.SHA384WithRSA:
		return jose.RS384, nil
	case x509.SHA512WithRSA:
		return jose.RS512, nil

	case x509.ECDSAWithSHA256:
		return jose.ES256, nil
	case x509.ECDSAWithSHA384:
		return jose.ES384, nil
	case x509.ECDSAWithSHA512:
		return jose.ES512, nil

	case x509.SHA256WithRSAPSS:
		return jose.PS256, nil
	case x509.SHA384WithRSAPSS:
		return jose.PS384, nil
	case x509.SHA512WithRSAPSS:
		return jose.PS512, nil

	case x509.PureEd25519:
		return jose.EdDSA, nil

	/*case x509.UnknownSignatureAlgorithm, x509.MD2WithRSA, x509.DSAWithSHA1, x509.DSAWithSHA256,
	x509.ECDSAWithSHA1, x509.SHA1WithRSA, x509.MD5WithRSA::*/
	default:
		return "", errors.New("unsupported certificate algorithm")
	}
}

type staticSymmetricKeys struct {
	key Key
}

func (s staticSymmetricKeys) Sign(context.Context) (Key, error) {
	return s.key, nil
}

func (s staticSymmetricKeys) Verify(context.Context) ([]Key, error) {
	return []Key{s.key}, nil
}

type staticAsymmetricKeys struct {
	pub  []Key
	priv Key
}

func (s staticAsymmetricKeys) Sign(context.Context) (Key, error) {
	return s.priv, nil
}

func (s staticAsymmetricKeys) Verify(context.Context) ([]Key, error) {
	return s.pub, nil
}
