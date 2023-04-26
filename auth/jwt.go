package auth

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gopkg.in/go-jose/go-jose.v2"
	"gopkg.in/go-jose/go-jose.v2/jwt"
)

// Role defines a user role.
type Role string

const (
	RoleFreeAccount Role = "free"
	RolePaidAccount Role = "superuser"
	RoleAdmin       Role = "god"

	defaultIssuer         = "hopper-auth"
	defaultExpireDuration = 24 * time.Hour
)

var (
	defaultAudience = []string{"hopper-user"}
)

// UserClaims represent the user claims.
type UserClaims struct {
	jwt.Claims
	Email  string `json:"email"`
	UserID string `json:"user_id"`
	Role   Role   `json:"role"`
}

// SignToken creates a new JWT token with the given user, email and role claims.
func SignToken(key []byte, userID, email string, role Role) (string, error) {
	claims := UserClaims{
		Email:  email,
		UserID: userID,
		Role:   role,
		Claims: jwt.Claims{
			Expiry:    jwt.NewNumericDate(time.Now().Add(defaultExpireDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    defaultIssuer,
			Subject:   userID,
			ID:        uuid.New().String(),
			Audience:  defaultAudience,
		},
	}

	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: key},
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

// VerifyToken verifies that the given token was signed by key
// and validates that it was generated using SignToken.
func VerifyToken(key []byte, token string) (UserClaims, error) {
	tok, err := jwt.ParseSigned(token)
	if err != nil {
		return UserClaims{}, fmt.Errorf("parse signed: %v", err)
	}

	var cl UserClaims
	if err := tok.Claims(key, &cl); err != nil {
		return UserClaims{}, fmt.Errorf("verify signature: %v", err)
	}

	expected := jwt.Expected{
		Issuer:   defaultIssuer,
		Audience: defaultAudience,
	}
	if err := cl.Validate(expected); err != nil {
		return UserClaims{}, fmt.Errorf("validate: %v", err)
	}

	if time.Now().After(cl.Expiry.Time()) {
		return UserClaims{}, fmt.Errorf("token expired on %s", cl.Expiry.Time().String())
	}

	return cl, nil
}
