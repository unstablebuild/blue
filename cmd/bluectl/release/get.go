// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
	if version == "latest" {
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
	defer f.Close()

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
