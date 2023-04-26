package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type User struct {
	Role string
}

func TestSignVerify(t *testing.T) {
	for _, secret := range [][]byte{nil, []byte("12345")} {
		token, err := SignToken(secret, "nada1234", "nada@unstable.build", User{Role: "admin"})
		require.NoError(t, err)

		claims, err := VerifyToken[User](secret, token)
		require.NoError(t, err)

		assert.Equal(t, "nada1234", claims.UserID)
		assert.Equal(t, "nada@unstable.build", claims.Email)
		assert.Equal(t, "admin", claims.Extra.Role)
	}
}
