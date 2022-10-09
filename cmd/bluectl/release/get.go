package release

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/crypto"
	"github.com/ernestrc/blue/release"
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
		Summary:  "Download a package bundle",
		Synopsis: "<package> <version> <out>",
		Options:  *s.fs,
	}
}

func (s *releaseGet) findKeyInArmoredKeyRing() (crypto.Key, error) {
	keys, err := crypto.FindKeysInArmoredKeyRing(s.keyring, s.pubKeyID, "")
	if err != nil {
		return crypto.Key{}, err
	}

	return keys[0], nil
}

func (s *releaseGet) Run(ctx context.Context, args []string) error {
	args, ok, err := cli.ParseUsage(s, s.fs, 3, args)
	if err != nil || !ok {
		return err
	}

	pack := args[0]
	version := release.Version(args[1])
	outfile := args[2]
	f, err := os.Create(outfile)
	if err != nil {
		return err
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, defaultGetTimeout)
	defer cancel()

	var m release.Bundle
	key, err := s.findKeyInArmoredKeyRing()
	if err != nil {
		fmt.Printf("WARNING: Failed to check data integrity: "+
			"error finding armored key '%s' in keyring: %s", s.pubKeyID, err)
		m, err = s.m.Get(ctx, pack, version, newBarProgress(f))
		if err != nil {
			return err
		}
	} else {
		sm := release.NewSigningManager(s.m, key)
		m, err = sm.Get(ctx, pack, version, newBarProgress(f))
		if err != nil {
			return err
		}
	}

	data, err := printableBundle(m)
	if err != nil {
		return err
	}
	fmt.Printf("\n---\n%+v\n", data)

	return nil
}
