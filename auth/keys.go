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
	//nolint:all
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"gopkg.in/go-jose/go-jose.v2"
)

// Key is one of the signing key types used by go-jose.
type Key struct {
	key  interface{}
	algo jose.SignatureAlgorithm
}

// Keys abstracts the ability to fetch signing/verification keys.
type Keys interface {
	Sign(context.Context) (Key, error)
	Verify(context.Context) ([]Key, error)
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
func SymmetricKey(data []byte) Key {
	return Key{key: data, algo: jose.HS256}
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
	case ed25519.PrivateKey:
		return jose.EdDSA
	default: // assume symmetric
		return jose.HS256
	}
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
