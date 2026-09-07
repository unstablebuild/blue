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

package release

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/crypto"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/blue/release/signedrelease"
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

func (s *releaseGet) getLatestVersion(
	ctx context.Context, pack string,
) (release.Version, error) {
	p, err := s.m.GetPackage(ctx, pack)
	if err != nil {
		return "", err
	}
	return p.Latest, nil
}

func (s *releaseGet) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 3, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultGetTimeout)
	defer cancel()

	pack := args[0]
	version := release.Version(args[1])
	if version == release.Latest {
		version, err = s.getLatestVersion(ctx, pack)
		if err != nil {
			return err
		}
		fmt.Printf("latest version is %q\n", version)
	}
	outfile := args[2]
	f, err := os.Create(outfile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

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
		sm := signedrelease.NewManager(s.m, key)
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
