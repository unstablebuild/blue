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

package signedrelease

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/unstablebuild/blue/crypto"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
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
	root release.Manager
	key  crypto.Key
}

// NewManager returns a Manager that wraps anoter manager to provide
// PGP signing of release artifacts with Create and PGP verification
// with Get requests.
func NewManager(other release.Manager, key crypto.Key) release.Manager {
	ret := new(signingManager)
	ret.root = other
	ret.key = key
	return ret
}

func (m *signingManager) Upload(
	ctx context.Context, man release.Bundle, in release.ProgressReader,
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
	progReader := release.NewRelayProgressReader(relayIn, in)
	return m.root.Upload(ctx, man, progReader)
}

func (m *signingManager) Get(
	ctx context.Context, pack string,
	ver release.Version, out release.ProgressWriter,
) (release.Bundle, error) {
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

	progWriter := release.NewRelayProgressWriter(relayIn, out)
	man, err := m.root.Get(ctx, pack, ver, progWriter)
	if err != nil {
		return release.Bundle{}, err
	}

	const templateMissingMetadata = "WARNING: Failed to check data integrity: " +
		"release artifact is missing %s in manifest metadata"
	signature, ok := man.Metadata[pgpSignedMetadata]
	if !ok {
		err := fmt.Errorf(templateMissingMetadata, pgpSignedMetadata)
		return release.Bundle{}, err
	}

	if isReadWriteSeeker {
		_, err = readWriteSeeker.Seek(0, 0)
		if err != nil {
			err = fmt.Errorf("failed to seek release artifact file: %s", err)
			return release.Bundle{}, err
		}
	}

	sig := strings.NewReader(signature)
	err = crypto.Verify(verifyReader, sig, m.key)
	if err != nil {
		return release.Bundle{}, err
	}

	return man, err
}

func (m *signingManager) Create(ctx context.Context, pack release.Package) error {
	return m.root.Create(ctx, pack)
}

func (m *signingManager) UpdatePackageMetadata(
	ctx context.Context, pack string, metadata map[string]string,
) error {
	return m.root.UpdatePackageMetadata(ctx, pack, metadata)
}

func (m *signingManager) Delete(
	ctx context.Context, pack string, ver release.Version,
) error {
	return m.root.Delete(ctx, pack, ver)
}

func (m *signingManager) List(
	ctx context.Context, pack string, filters map[string]string,
) (iterator.Iterator[release.Bundle], error) {
	return m.root.List(ctx, pack, filters)
}

func (m *signingManager) ListPackages(
	ctx context.Context, filters map[string]string,
) (iterator.Iterator[release.Package], error) {
	return m.root.ListPackages(ctx, filters)
}

func (m *signingManager) DeletePackage(
	ctx context.Context, pack string,
) error {
	return m.root.DeletePackage(ctx, pack)
}

func (m *signingManager) GetPackage(
	ctx context.Context, pack string,
) (release.Package, error) {
	return m.root.GetPackage(ctx, pack)
}
