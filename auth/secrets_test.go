package auth

import (
	"context"
	"strconv"
	"testing"

	"github.com/ernestrc/blue/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	ctx := context.Background()

	t.Run("secrets are verifiable", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewStore(svc)

		myMetadata := map[string]string{"blah": "bleh"}
		err := s.CreateSecret(ctx, "myId", []byte("1234"), myMetadata)
		require.NoError(t, err)

		actualMeta, err := s.VerifySecret(ctx, "myId", []byte("1234"))
		require.NoError(t, err)
		assert.Equal(t, myMetadata, actualMeta)
	})

	t.Run("returns ErrInvalidData if secret doesn't exist", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewStore(svc)

		err := s.CreateSecret(ctx, "myId", []byte("1234"), nil)
		require.NoError(t, err)

		_, err = s.VerifySecret(ctx, "myId", []byte("1235"))
		require.Equal(t, ErrInvalidData, err)
	})

	t.Run("secrets cannot be retrieved from the store", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewStore(svc)

		err := s.CreateSecret(ctx, "myId", []byte("1234"), nil)
		require.NoError(t, err)

		var sec secret
		err = s.svc.Get(ctx, "myId", &sec)
		require.NoError(t, err)
		assert.NotEqual(t, []byte("1234"), sec.Data)
	})

	t.Run("CreateSecret returns error if secret with given ID has already been created", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewStore(svc)

		err := s.CreateSecret(ctx, "myId", []byte("1234"), nil)
		require.NoError(t, err)

		err = s.CreateSecret(ctx, "myId", []byte("2346"), nil)
		require.Error(t, err)
	})

	t.Run("ListSecrets returns all the secrets stored", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewStore(svc)

		for i := 0; i < 10; i++ {
			require.NoError(t, s.CreateSecret(ctx, strconv.Itoa(i), []byte(strconv.Itoa(i)), map[string]string{
				strconv.Itoa(i): strconv.Itoa(i),
			}))
		}

		it, err := s.ListSecrets(ctx, nil)
		require.NoError(t, err)

		var i int
		for {
			sec, ok := it.Next()
			if !ok {
				break
			}
			assert.Equal(t, sec.ID, sec.Metadata[sec.ID])
			assert.NotZero(t, sec.UpdatedAt)
			assert.NotZero(t, sec.CreatedAt)
			i++
		}

		require.NoError(t, it.Err())
		require.Equal(t, 10, i)
	})

	t.Run("Revoke revokes a secret and future calls to IsValid return false", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewStore(svc)

		err := s.CreateSecret(ctx, "myId", []byte("1234"), nil)
		require.NoError(t, err)

		require.NoError(t, s.Revoke(ctx, "myId"))

		_, err = s.VerifySecret(ctx, "myId", []byte("1234"))
		require.Equal(t, ErrRevoked, err)
	})

	t.Run("Revoke returns ErrNotFound if trying to revoke a secret that doesn't exist", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewStore(svc)

		require.Equal(t, ErrNotFound, s.Revoke(ctx, "myId"))
	})

	t.Run("ListSecrets returns only non-revoked secrets", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewStore(svc)

		for i := 0; i < 10; i++ {
			require.NoError(t, s.CreateSecret(ctx, strconv.Itoa(i), []byte(strconv.Itoa(i)), map[string]string{
				strconv.Itoa(i): strconv.Itoa(i),
			}))
			require.NoError(t, s.Revoke(ctx, strconv.Itoa(i)))
		}

		require.NoError(t, s.CreateSecret(ctx, "1234", []byte("1234"), nil))

		it, err := s.ListSecrets(ctx, map[string]string{"Revoked": strconv.FormatBool(false)})
		require.NoError(t, err)

		var i int
		for {
			sec, ok := it.Next()
			if !ok {
				break
			}
			assert.Equal(t, "1234", sec.ID)
			i++
		}

		require.NoError(t, it.Err())
		require.Equal(t, 1, i)
	})
}
