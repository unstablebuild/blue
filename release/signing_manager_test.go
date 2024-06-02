package release

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/crypto"
	cryptest "github.com/unstablebuild/blue/crypto/test"
)

func makeReleaseContent(t *testing.T, content string) *os.File {
	f, err := os.CreateTemp("", "")
	require.NoError(t, err)

	_, err = io.Copy(f, strings.NewReader(content))
	require.NoError(t, err)

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	return f
}

func signReleaseContent(
	t *testing.T, key crypto.Key, man Bundle, outContent, signedContent string,
) func(ctx context.Context, Package string, ver Version, in io.Writer) (Bundle, error) {
	return func(ctx context.Context, pack string, ver Version, in io.Writer) (
		Bundle, error,
	) {
		assert.Equal(t, man.Package, pack)
		assert.Equal(t, man.Version, ver)

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

func expectUpload(
	t *testing.T, mock *MockManager, key crypto.Key, man Bundle,
	expectStat bool,
) {
	mock.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _man Bundle, in io.Reader) error {
			assert.Equal(t, man.Package, _man.Package)
			assert.Equal(t, man.Version, _man.Version)
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
	man := Bundle{
		Package: "bla",
		Version: "blo",
		Notes:   "blublu",
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
		expectUpload(t, mock, key, man, true)

		err := m.Upload(ctx, man, in)
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

		expectUpload(t, mock, key, man, true)

		err := m.Upload(ctx, man, NopProgressReader(in))
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

		err := m.Upload(ctx, man, NopProgressReader(in))
		require.Equal(t, ErrEncryptedKey, err)
	})

	t.Run("Upload returns underlying manager error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)
		in := makeReleaseContent(t, "T****")
		defer in.Close()

		mock.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("capitol insurrectionists")).Times(1)

		err := m.Upload(ctx, man, NopProgressReader(in))
		assert.Error(t, err)
	})

	t.Run("verifies releases upon calls to Get in buffered mode", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)
		contentStr := "super buffered important data"

		mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(signReleaseContent(t, key, man, contentStr, contentStr)).Times(1)

		var out bytes.Buffer
		ret, err := m.Get(ctx, man.Package, man.Version, NopProgressWriter(&out))
		require.NoError(t, err)
		assert.Equal(t, man.Package, ret.Package)
		assert.Equal(t, man.Version, ret.Version)
		assert.Equal(t, man.Notes, ret.Notes)

		assert.Equal(t, contentStr, out.String())
	})

	t.Run("verifies releases upon calls to Get in non buffered mode", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)
		contentStr := "super important data"

		mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(signReleaseContent(t, key, man, contentStr, contentStr)).Times(1)

		out := makeReleaseContent(t, "")
		defer out.Close()
		ret, err := m.Get(ctx, man.Package, man.Version, NopProgressWriter(out))
		require.NoError(t, err)
		assert.Equal(t, man.Package, ret.Package)
		assert.Equal(t, man.Version, ret.Version)
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

		mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(Bundle{}, errors.New("oopsie daisy")).Times(1)

		_, err := m.Get(ctx, man.Package, man.Version,
			NopProgressWriter(io.Discard))
		require.Error(t, err)
	})

	t.Run("Get returns error if signature is invalid", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)

		mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(signReleaseContent(t, key, man, "something REAL bad", "something good")).
			Times(1)

		out := makeReleaseContent(t, "")
		defer out.Close()
		_, err := m.Get(ctx, man.Package, man.Version, NopProgressWriter(out))
		require.Error(t, err)
	})

	t.Run("Get returns error if signature is missing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := NewMockManager(ctrl)
		m := NewSigningManager(mock, key)
		pack := "myID"
		ver := Version("1.0.0")

		mock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ string, _ Version, in io.Writer) (Bundle, error) {
				return Bundle{Package: pack, Version: ver}, nil
			}).
			Times(1)

		out := makeReleaseContent(t, "")
		defer out.Close()
		_, err := m.Get(ctx, man.Package,
			man.Version, NopProgressWriter(out))
		require.Error(t, err)
	})
}
