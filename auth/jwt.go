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
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gopkg.in/go-jose/go-jose.v2"
	"gopkg.in/go-jose/go-jose.v2/jwt"
)

const (
	defaultIssuer = "blue-auth"
)

var (
	defaultAudience = []string{"blue-user"}
)

// UserClaims represent the user claims.
type UserClaims[T any] struct {
	jwt.Claims
	Email  string `json:"email"`
	UserID string `json:"user_id"`
	Extra  T      `json:"extra"`
}

// SignToken creates a new JWT token with the given user, email and role claims.
func SignToken[T any](key Key, userID, email string, extraClaims T, expiry time.Duration) (string, error) {
	claims := UserClaims[T]{
		Email:  email,
		UserID: userID,
		Extra:  extraClaims,
		Claims: jwt.Claims{
			Expiry:    jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    defaultIssuer,
			Subject:   userID,
			ID:        uuid.New().String(),
			Audience:  defaultAudience,
		},
	}

	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: key.algo, Key: key.key},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return "", fmt.Errorf("new jwt signer: %v", err)
	}
	raw, err := jwt.Signed(sig).Claims(claims).CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("sign token: %v", err)
	}
	return raw, nil
}

// VerifyToken verifies that the given token was signed by key.
func VerifyToken[T any](key Key, token string) (UserClaims[T], error) {
	tok, err := jwt.ParseSigned(token)
	if err != nil {
		return UserClaims[T]{}, fmt.Errorf("parse signed: %v", err)
	}

	log.Debugf("verifying token with headers: %v", tok.Headers)

	var cl UserClaims[T]
	if err := tok.Claims(key.key, &cl); err != nil {
		return UserClaims[T]{}, fmt.Errorf("verify signature: %v", err)
	}

	if time.Now().After(cl.Expiry.Time()) {
		return UserClaims[T]{}, fmt.Errorf("token expired on %s", cl.Expiry.Time().String())
	}

	return cl, nil
}
