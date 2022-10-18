package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ernestrc/blue/crypto"
	"github.com/ernestrc/blue/debug"
)

// ErrEncryptedKey is returned when provided key is encrypted and needs decrypting first.
var ErrEncryptedKey = errors.New("provided PGP key is encrypted. " +
	"Decrypt first before using it")

const (
	pgpSignedMetadata         = "pgp-signature"
	pgpSignedMetadataKey      = "pgp-signing-key-id"
	pgpSignedMetadataIdentity = "pgp-signing-primary-identity"
)

type signingManager struct {
	root Manager
	key  crypto.Key
}

// NewSigningManager returns a Manager that wraps anoter manager to provide
// PGP signing of release artifacts with Create and PGP verification
// with Get requests.
func NewSigningManager(other Manager, key crypto.Key) Manager {
	ret := new(signingManager)
	ret.root = other
	ret.key = key
	return ret
}

func (m *signingManager) Upload(
	ctx context.Context, man Bundle, in ProgressReader,
) error {
	var out bytes.Buffer
	var relayIn io.Reader
	var inArmored io.Reader = in

	// if in is seeker, then avoid buffering and instead reset reader
	// before Create call to underlying Manager
	seeker, isSeeker := in.(io.Seeker)
	if isSeeker {
		relayIn = in
	} else {
		var buf bytes.Buffer
		inArmored = io.TeeReader(in, &buf)
		relayIn = &buf
	}

	err := crypto.ArmoredSign(inArmored, &out, m.key)
	if err != nil {
		if !strings.Contains(err.Error(), "signing key is encrypted") {
			err = fmt.Errorf("failed to sign release artifact with PGP key: %s", err)
			return err
		}
		return ErrEncryptedKey
	}

	if man.Metadata == nil {
		man.Metadata = make(map[string]string)
	}
	man.Metadata[pgpSignedMetadataIdentity] = m.key.PrimaryIdentity().Name
	man.Metadata[pgpSignedMetadata] = out.String()
	man.Metadata[pgpSignedMetadataKey] = m.key.PrivateKey.KeyIdString()

	if isSeeker {
		_, err = seeker.Seek(0, 0)
		if err != nil {
			err = fmt.Errorf("failed to seek release artifact: %s", err)
			return err
		}
	}
	progReader := newRelayProgressReader(in, relayIn)
	return m.root.Upload(ctx, man, progReader)
}

func (m *signingManager) Get(
	ctx context.Context, pack string,
	ver Version, out ProgressWriter,
) (Bundle, error) {
	var relayIn io.Writer
	var verifyReader io.Reader

	// if out is read write seeker, then avoid buffering and instead reset it
	// before verifying signature
	readWriteSeeker, isReadWriteSeeker := out.(io.ReadWriteSeeker)
	if isReadWriteSeeker {
		relayIn = readWriteSeeker
		verifyReader = readWriteSeeker
	} else {
		var buf bytes.Buffer
		verifyReader = io.TeeReader(&buf, out)
		relayIn = &buf
	}

	progWriter := progressDelegate{writeDelegate: relayIn, progressDelegate: out}
	man, err := m.root.Get(ctx, pack, ver, progWriter)
	if err != nil {
		return Bundle{}, err
	}

	const templateMissingMetadata = "WARNING: Failed to check data integrity: " +
		"release artifact is missing %s in manifest metadata"
	signature, ok := man.Metadata[pgpSignedMetadata]
	if !ok {
		err := fmt.Errorf(templateMissingMetadata, pgpSignedMetadata)
		return Bundle{}, err
	}

	if isReadWriteSeeker {
		_, err = readWriteSeeker.Seek(0, 0)
		if err != nil {
			err = fmt.Errorf("failed to seek release artifact file: %s", err)
			return Bundle{}, err
		}
	}

	sig := strings.NewReader(signature)
	err = crypto.Verify(verifyReader, sig, m.key)
	if err != nil {
		return Bundle{}, err
	}

	return man, err
}

func (m *signingManager) Create(ctx context.Context, pack Package) error {
	return m.root.Create(ctx, pack)
}

func (m *signingManager) Delete(
	ctx context.Context, pack string, ver Version,
) error {
	return m.root.Delete(ctx, pack, ver)
}

func (m *signingManager) List(
	ctx context.Context, pack string, filters map[string]string,
) ([]Bundle, error) {
	return m.root.List(ctx, pack, filters)
}

func (m *signingManager) ListPackages(
	ctx context.Context, filters map[string]string,
) ([]Package, error) {
	return m.root.ListPackages(ctx, filters)
}

func (m *signingManager) DeletePackage(
	ctx context.Context, pack string,
) error {
	return m.root.DeletePackage(ctx, pack)
}

func (m *signingManager) GetPackage(
	ctx context.Context, pack string,
) (Package, error) {
	return m.root.GetPackage(ctx, pack)
}

func (m *signingManager) AddPanicReport(ctx context.Context, r debug.PanicReport) error {
	return m.root.AddPanicReport(ctx, r)
}

func (m *signingManager) ListPanicReports(ctx context.Context, pkg, ver string,
	filters map[string]string) ([]debug.PanicReport, error) {
	return m.root.ListPanicReports(ctx, pkg, ver, filters)
}

func (m *signingManager) GetPanicReport(ctx context.Context, id string) (debug.PanicReport, error) {
	return m.root.GetPanicReport(ctx, id)
}
