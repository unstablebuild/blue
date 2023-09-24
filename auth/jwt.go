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
