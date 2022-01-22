package release

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/ernestrc/blue/crypto"
	cryptest "github.com/ernestrc/blue/crypto/test"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeReleaseContent(t *testing.T, content string) *os.File {
	f, err := ioutil.TempFile("", "")
	require.NoError(t, err)

	_, err = io.Copy(f, strings.NewReader(content))
	require.NoError(t, err)

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	return f
}

func signReleaseContent(
	t *testing.T, key crypto.Key, man Manifest, outContent, signedContent string,
) func(ctx context.Context, ID string, in io.Writer) (Manifest, error) {
	return func(ctx context.Context, ID string, in io.Writer) (Manifest, error) {
		assert.Equal(t, ID, ID)

		ret := man
		ret.Metadata = make(map[string]string)

		_, err := io.Copy(in, strings.NewReader(outContent))
		require.NoError(t, err)

		var out bytes.Buffer
		require.NoError(t, crypto.ArmoredSign(strings.NewReader(signedContent), &out, key))

		ret.Metadata[pgpSignedMetadata] = out.String()

		return ret, nil
	}
}

func expectCreate(
	t *testing.T, mock *MockManager, key crypto.Key, man Manifest,
	expectStat bool,
) {
	mock.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _man Manifest, in io.Reader) error {
			assert.Equal(t, man.ID, _man.ID)
			assert.Equal(t, man.Notes, _man.Notes)
			require.NotNil(t, _man.Metadata)

			signatureStr, ok := _man.Metadata[pgpSignedMetadata]
			require.True(t, ok)
			assert.NotZero(t, signatureStr)

			signature := strings.NewReader(signatureStr)

			require.NoError(t, crypto.Verify(in, signature, key))

			_, ok = in.(interface{ Stat() (os.FileInfo, error) })
			assert.Equal(t, expectStat, ok)

			return nil
		}).Times(1)
}

func TestSigningManager(t *testing.T) {
	ctx := context.Background()
	key := crypto.Key(cryptest.GenerateTestKey(t))
	man := Manifest{
		ID:    "bla",
		Notes: "blabla",
	}

	t.Run("creates a signed release with in buffered mode", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)

		// does not satisfy io.Seeker
		in := NopProgressReader(strings.NewReader("we want buffered I/O!"))

		// NOTE: last arg should be false.
		// Should refactor delegate to install delegate without
		// Stat if input does not satisfy os.Stat, so implementations
		// can still use interface tests
		expectCreate(t, mock, key, man, true)

		err := m.Create(ctx, man, in)
		require.NoError(t, err)
	})

	t.Run("creates a signed release in non-buffered mode", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)

		// satisfies io.Seeker
		in := makeReleaseContent(t, "wasup")
		defer in.Close()

		expectCreate(t, mock, key, man, true)

		err := m.Create(ctx, man, NopProgressReader(in))
		require.NoError(t, err)
	})

	t.Run("creates returns ErrEncryptedKey if key supplied is encrypted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		encryptedKey := crypto.Key(cryptest.GenerateTestKey(t))
		encryptedKey.Entity.PrivateKey.Encrypted = true
		m := NewSigningManager(mock, encryptedKey)
		in := makeReleaseContent(t, "wasup")
		defer in.Close()

		err := m.Create(ctx, man, NopProgressReader(in))
		require.Equal(t, ErrEncryptedKey, err)
	})

	t.Run("Create returns underlying manager error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)
		in := makeReleaseContent(t, "T****")
		defer in.Close()

		mock.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("capitol insurrectionists")).Times(1)

		err := m.Create(ctx, man, NopProgressReader(in))
		assert.Error(t, err)
	})

	t.Run("verifies releases upon calls to Get in buffered mode", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)
		contentStr := "super buffered important data"

		mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(signReleaseContent(t, key, man, contentStr, contentStr)).Times(1)

		var out bytes.Buffer
		ret, err := m.Get(ctx, man.ID, NopProgressWriter(&out))
		require.NoError(t, err)
		assert.Equal(t, man.ID, ret.ID)
		assert.Equal(t, man.Notes, ret.Notes)

		assert.Equal(t, contentStr, out.String())
	})

	t.Run("verifies releases upon calls to Get in non buffered mode", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)
		contentStr := "super important data"

		mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(signReleaseContent(t, key, man, contentStr, contentStr)).Times(1)

		out := makeReleaseContent(t, "")
		defer out.Close()
		ret, err := m.Get(ctx, man.ID, NopProgressWriter(out))
		require.NoError(t, err)
		assert.Equal(t, man.ID, ret.ID)
		assert.Equal(t, man.Notes, ret.Notes)

		_, err = out.Seek(0, 0)
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = io.Copy(&buf, out)
		require.NoError(t, err)

		assert.Equal(t, contentStr, buf.String())
	})

	t.Run("Get bubbles up error from underlying manager", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)

		mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(Manifest{}, errors.New("oopsie daisy")).Times(1)

		_, err := m.Get(ctx, man.ID, NopProgressWriter(ioutil.Discard))
		require.Error(t, err)
	})

	t.Run("Get returns error if signature is invalid", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)

		mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(signReleaseContent(t, key, man, "something REAL bad", "something good")).
			Times(1)

		out := makeReleaseContent(t, "")
		defer out.Close()
		_, err := m.Get(ctx, man.ID, NopProgressWriter(out))
		require.Error(t, err)
	})

	t.Run("Get returns error if signature is missing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)
		id := "myID"

		mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, ID string, in io.Writer) (Manifest, error) {
				return Manifest{ID: id}, nil
			}).
			Times(1)

		out := makeReleaseContent(t, "")
		defer out.Close()
		_, err := m.Get(ctx, man.ID, NopProgressWriter(out))
		require.Error(t, err)
	})
}
