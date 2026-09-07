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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"golang.org/x/oauth2/authhandler"
)

const (
	defaultLength = 32
	minLength     = 32
	maxLength     = 96
)

func createCodeVerifierWithLength(length int) (string, error) {
	if length < minLength || length > maxLength {
		return "", fmt.Errorf("invalid length: %v", length)
	}
	buf, err := randomBytes(length)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %v", err)
	}
	return createCodeVerifierFromBytes(buf), nil
}

func createCodeVerifierFromBytes(b []byte) string {
	return encode(b)
}

func codeChallengeS256(in string) string {
	h := sha256.New()
	h.Write([]byte(in))
	return encode(h.Sum(nil))
}

func encode(msg []byte) string {
	encoded := base64.StdEncoding.EncodeToString(msg)
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.ReplaceAll(encoded, "=", "")
	return encoded
}

// https://github.com/nirasan/go-oauth-pkce-code-verifier/blob/master/verifier.go
func randomBytes(length int) ([]byte, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const csLen = byte(len(charset))
	output := make([]byte, 0, length)
	for {
		buf := make([]byte, length)
		if _, err := io.ReadFull(rand.Reader, buf); err != nil {
			return nil, fmt.Errorf("failed to read random bytes: %v", err)
		}
		for _, b := range buf {
			// Avoid bias by using a value range that's a multiple of 62
			if b < (csLen * 4) {
				output = append(output, charset[b%csLen])
				if len(output) == length {
					return output, nil
				}
			}
		}
	}
}

func generatePKCEParams(length int) (authhandler.PKCEParams, error) {
	verifier, err := createCodeVerifierWithLength(length)
	if err != nil {
		return authhandler.PKCEParams{}, fmt.Errorf("create code verifier: %v", err)
	}
	challenge := codeChallengeS256(verifier)
	return authhandler.PKCEParams{
		Challenge:       challenge,
		ChallengeMethod: "S256",
		Verifier:        verifier,
	}, nil
}
