package release

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/crypto"
	"github.com/ernestrc/blue/release"
	log "github.com/sirupsen/logrus"
)

const (
	defaultGetTimeout = 10 * time.Minute
)

type releaseGet struct {
	m        release.Manager
	fs       *cli.FlagSet
	keyring  string
	pubKeyID string
}

func newReleaseGetCLI(m release.Manager) cli.CLI {
	g := &releaseGet{
		m: m,
	}
	g.fs = cli.NewFlagSet("get")
	g.fs.StringVar(&g.pubKeyID, "k", "", "Verify release with PGP public key. "+
		"Default is to use the key ID in the release manifest.")
	g.fs.StringVar(&g.keyring, "r", "pubring.gpg",
		"Armored keyring to search fo PGP key used to sign release artifact.")
	return g
}

func (s *releaseGet) Man() cli.Manual {
	return cli.Manual{
		Name:     "get",
		Summary:  "Download a release by tag",
		Synopsis: "<tag> <out>",
		Options:  *s.fs,
	}
}

func (s *releaseGet) verifySignature(in *os.File, keyID, signature string) error {
	if s.pubKeyID != "" {
		keyID = s.pubKeyID
	}

	keys, err := crypto.FindKeysInArmoredKeyRing(s.keyring, keyID, "")
	if err != nil {
		return err
	}

	key := keys[0]
	sig := strings.NewReader(signature)
	log.Debugf("verifying release PGP signature with public keyring %s and key ID %s: %s",
		s.keyring, keyID, signature)

	_, err = in.Seek(0, 0)
	if err != nil {
		err = fmt.Errorf("failed to seek release artifact file: %s", err)
		return err
	}

	return crypto.Verify(in, sig, key)
}

func (s *releaseGet) Run(ctx context.Context, args []string) error {
	args, _, err := cli.Parse(s.fs, 2, args)
	if err != nil {
		if err == cli.ErrHelp || err == cli.ErrInvalidArgs {
			cli.Usage(s)
			err = nil
		}
		return err
	}

	id := args[0]
	outfile := args[1]
	f, err := os.Create(outfile)
	if err != nil {
		return err
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, defaultGetTimeout)
	defer cancel()

	m, err := s.m.Get(ctx, id, f)
	if err != nil {
		return err
	}

	const templateMissingMetadata = "WARNING: Failed to check data integrity: " +
		"release artifact is missing %s in manifest metadata"
	signature, ok := m.Metadata[pgpSignedMetadata]
	if !ok {
		err := fmt.Errorf(templateMissingMetadata, pgpSignedMetadata)
		return err
	}

	keyID, ok := m.Metadata[pgpSignedMetadataKey]
	if !ok {
		err := fmt.Errorf(templateMissingMetadata, pgpSignedMetadataKey)
		return err
	}

	err = s.verifySignature(f, keyID, signature)
	if err != nil {
		return err
	}

	data, err := printableManifest(m)
	if err != nil {
		return err
	}
	fmt.Printf("\n---\n%+v\n", data)

	return nil
}
