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

	t.Run("passwords are verifiable", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewPasswordStore(svc)

		myMetadata := map[string]string{"blah": "bleh"}
		err := s.CreatePassword(ctx, "myId", []byte("1234"), myMetadata)
		require.NoError(t, err)

		actualMeta, err := s.VerifyPassword(ctx, "myId", []byte("1234"))
		require.NoError(t, err)
		assert.Equal(t, myMetadata, actualMeta)
	})

	t.Run("returns ErrInvalidData if secret doesn't exist", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewPasswordStore(svc)

		err := s.CreatePassword(ctx, "myId", []byte("1234"), nil)
		require.NoError(t, err)

		_, err = s.VerifyPassword(ctx, "myId", []byte("1235"))
		require.Equal(t, ErrInvalidData, err)
	})

	t.Run("passwords cannot be retrieved from the store", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewPasswordStore(svc)

		err := s.CreatePassword(ctx, "myId", []byte("1234"), nil)
		require.NoError(t, err)

		var sec password
		err = s.svc.Get(ctx, "myId", &sec)
		require.NoError(t, err)
		assert.NotEqual(t, []byte("1234"), sec.Data)
	})

	t.Run("CreatePassword returns error if secret with given ID has already been created", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewPasswordStore(svc)

		err := s.CreatePassword(ctx, "myId", []byte("1234"), nil)
		require.NoError(t, err)

		err = s.CreatePassword(ctx, "myId", []byte("2346"), nil)
		require.Error(t, err)
	})

	t.Run("ListPasswords returns all the passwords stored", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewPasswordStore(svc)

		for i := 0; i < 10; i++ {
			require.NoError(t, s.CreatePassword(ctx, strconv.Itoa(i), []byte(strconv.Itoa(i)), map[string]string{
				strconv.Itoa(i): strconv.Itoa(i),
			}))
		}

		it, err := s.ListPasswords(ctx, nil)
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
		s := NewPasswordStore(svc)

		err := s.CreatePassword(ctx, "myId", []byte("1234"), nil)
		require.NoError(t, err)

		require.NoError(t, s.Revoke(ctx, "myId"))

		_, err = s.VerifyPassword(ctx, "myId", []byte("1234"))
		require.Equal(t, ErrRevoked, err)
	})

	t.Run("Revoke returns ErrNotFound if trying to revoke a secret that doesn't exist", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewPasswordStore(svc)

		require.Equal(t, ErrNotFound, s.Revoke(ctx, "myId"))
	})

	t.Run("ListPasswords returns only non-revoked passwords ", func(t *testing.T) {
		svc := document.NewInMemoryService()
		s := NewPasswordStore(svc)

		for i := 0; i < 10; i++ {
			require.NoError(t, s.CreatePassword(ctx, strconv.Itoa(i), []byte(strconv.Itoa(i)), map[string]string{
				strconv.Itoa(i): strconv.Itoa(i),
			}))
			require.NoError(t, s.Revoke(ctx, strconv.Itoa(i)))
		}

		require.NoError(t, s.CreatePassword(ctx, "1234", []byte("1234"), nil))

		it, err := s.ListPasswords(ctx, map[string]string{"Revoked": strconv.FormatBool(false)})
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
