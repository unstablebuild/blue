package release

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/cmd/bluectl/options"
	"github.com/ernestrc/blue/crypto"
	"github.com/ernestrc/blue/release"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	createTimeout             = 10 * time.Minute
	pgpSignedMetadata         = "pgp-signature"
	pgpSignedMetadataKey      = "pgp-signing-key-id"
	pgpSignedMetadataIdentity = "pgp-signing-primary-identity"
)

type releaseCreate struct {
	m           release.Manager
	fs          *cli.FlagSet
	privKeyID   string
	keyRingFile string
}

func getDefaultAuthor() string {
	u, err := user.Current()
	if err != nil {
		u = &user.User{Username: "unknown"}
	}
	h, err := os.Hostname()
	if err != nil {
		h = "unknown-host"
	}
	return fmt.Sprintf("%s@%s", u.Username, h)
}

func newReleaseCreateCLI(m release.Manager) cli.CLI {
	c := &releaseCreate{
		m: m,
	}
	c.fs = cli.NewFlagSet("create")
	c.fs.StringVar(&c.privKeyID, "k", "", "Sign release with PGP private key. "+
		"This forces clients to provide a public key upon downloading release.")
	c.fs.StringVar(&c.keyRingFile, "r", "secring.gpg", "Armored keyring file to use to find private key.")
	return c
}

func (s *releaseCreate) Man() cli.Manual {
	return cli.Manual{
		Name:     "create",
		Summary:  "Create a release with the given tag and tar file",
		Synopsis: "<tag> <filename>",
		Options:  *s.fs,
	}
}

func findPrivateKeyInKeyRing(keyRingFile, privKeyID, passphrase string) (crypto.Key, error) {
	keys, err := crypto.FindKeysInArmoredKeyRing(keyRingFile, privKeyID, passphrase)
	if err != nil {
		return crypto.Key{}, err
	}

	for _, k := range keys {
		if k.PrivateKey != nil {
			return k, nil
		}
	}

	err = fmt.Errorf("failed to find private PGP key to sign release")
	return crypto.Key{}, err
}

func readPasswordFromStdin() (string, error) {
	fmt.Fprintf(os.Stdout, "PGP key passphrase:")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		err = fmt.Errorf("failed to read passphrase from stdin: %s", err)
		return "", err
	}
	fmt.Fprintf(os.Stdout, "\r")
	return string(bytePassword), nil
}

func (s *releaseCreate) signWithPrivKey(in *os.File, m *release.Manifest) (err error) {
	var out bytes.Buffer
	var passphrase string
	var key crypto.Key
	for {
		_, err = in.Seek(0, 0)
		if err != nil {
			err = fmt.Errorf("failed to seek release artifact file: %s", err)
			return err
		}

		log.Debugf("searcing key %s in keyring %s", s.privKeyID, s.keyRingFile)

		key, err = findPrivateKeyInKeyRing(s.keyRingFile, s.privKeyID, passphrase)
		if err != nil {
			return err
		}

		out.Reset()
		err = crypto.ArmoredSign(in, &out, key)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "signing key is encrypted") {
			err = fmt.Errorf("failed to sign release artifact with PGP key: %s", err)
			return err
		}

		log.Debugf("found key %s but requires passphrase", s.privKeyID)

		passphrase, err = readPasswordFromStdin()
		if err != nil {
			return
		}
	}

	_, err = in.Seek(0, 0)
	if err != nil {
		err = fmt.Errorf("failed to seek release artifact file: %s", err)
		return err
	}

	m.Metadata[pgpSignedMetadataIdentity] = key.PrimaryIdentity().Name
	m.Metadata[pgpSignedMetadata] = out.String()
	m.Metadata[pgpSignedMetadataKey] = key.PrivateKey.KeyIdString()
	return nil
}

func (s *releaseCreate) Run(ctx context.Context, args []string) error {
	args, _, err := cli.Parse(s.fs, 2, args)
	if err != nil {
		if err == cli.ErrHelp || err == cli.ErrInvalidArgs {
			cli.Usage(s)
			err = nil
		}
		return err
	}

	file, err := options.OpenFile(args[1])
	if err != nil {
		return err
	}
	defer file.Close()

	m, err := tempManifest(args[0], getDefaultAuthor())
	if err != nil {
		return err
	}

	if s.privKeyID != "" {
		err = s.signWithPrivKey(file, &m)
		if err != nil {
			return err
		}
		log.Debugf("added release PGP signature with private key: %s: %s",
			s.privKeyID, m.Metadata[pgpSignedMetadata])
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	return s.m.Create(ctx, m, file)
}
