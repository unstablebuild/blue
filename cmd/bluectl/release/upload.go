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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cmd/bluectl/options"
	"github.com/unstablebuild/blue/crypto"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/blue/release/signedrelease"
	"golang.org/x/term"
)

const (
	uploadTimeout = 10 * time.Minute
)

type metadataFlag []string

func (i *metadataFlag) String() string {
	return ""
}

func (i *metadataFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type releaseUpload struct {
	m                 release.Manager
	fs                *cli.FlagSet
	privKeyID         string
	privKeyPassphrase string
	keyRingFile       string
	noEdit            bool
	mdataFlag         metadataFlag
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

func newReleaseUploadCLI(m release.Manager) cli.CLI {
	c := &releaseUpload{
		m: m,
	}
	c.fs = cli.NewFlagSet("upload")
	c.fs.StringVar(&c.privKeyID, "k", "", "Sign release with PGP private key. "+
		"This forces clients to provide a public key upon downloading release.")
	c.fs.BoolVar(&c.noEdit, "y", false, "Do not prompt user to edit file manifest.")
	c.fs.StringVar(&c.privKeyPassphrase, "p", "", "Use the given passphrase rather than prompt user via stdio.")
	c.fs.StringVar(&c.keyRingFile, "r", "secring.gpg",
		"Armored keyring file to use to find private key.")
	c.fs.Var(&c.mdataFlag, "d", "Add default metadata to manifest. Expects format to be <key>=<value>")
	return c
}

func (s *releaseUpload) Man() cli.Manual {
	return cli.Manual{
		Name:     "upload",
		Summary:  "Upload a package bundle with the given tag and tar file",
		Synopsis: "<package> <version> <filename>",
		Options:  *s.fs,
	}
}

func findPrivateKeyInKeyRing(
	keyRingFile, privKeyID, passphrase string,
) (crypto.Key, error) {
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
	_, _ = fmt.Fprintf(os.Stdout, "PGP key passphrase:")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		err = fmt.Errorf("failed to read passphrase from stdin: %s", err)
		return "", err
	}
	_, _ = fmt.Fprintf(os.Stdout, "\r")
	return string(bytePassword), nil
}

func (s *releaseUpload) uploadSignedRelease(
	ctx context.Context, m release.Bundle, in *os.File,
) error {
	var out bytes.Buffer
	var passphrase string
	var key crypto.Key
	var pb *barProgress
	defer func() {
		if pb != nil {
			_ = pb.Close()
		}
	}()
	for {
		_, err := in.Seek(0, 0)
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

		sm := signedrelease.NewManager(s.m, key)

		ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
		defer cancel()

		pb = newBarProgress(in)

		err = sm.Upload(ctx, m, pb)
		if err == nil {
			return nil
		}
		if err != signedrelease.ErrEncryptedKey {
			return err
		}

		log.Debugf("found key %s but requires passphrase", s.privKeyID)

		if s.privKeyPassphrase == "" {
			passphrase, err = readPasswordFromStdin()
			if err != nil {
				return err
			}
		} else {
			passphrase = s.privKeyPassphrase
		}

		_ = pb.Close()
		pb = nil
	}
}

func (s *releaseUpload) parseMetadataFlag() (map[string]string, error) {
	ret := make(map[string]string)
	for _, arg := range s.mdataFlag {
		kv := strings.Split(arg, "=")
		if len(kv) != 2 {
			return nil, errors.New("invalid -d format")
		}
		ret[kv[0]] = kv[1]
	}
	return ret, nil
}

func (s *releaseUpload) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 3, args)
	if err != nil || !ok {
		return err
	}
	pack := args[0]
	version := release.Version(args[1])
	if version == "latest" {
		return errors.New("'latest' is a reserved version, automatically set to the latest uploaded bundle")
	}
	out := args[2]

	file, err := options.OpenFile(out)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	mdata, err := s.parseMetadataFlag()
	if err != nil {
		cli.Usage(s)
		return err
	}

	m, err := tempBundle(pack, version, getDefaultAuthor(), mdata, !s.noEdit)
	if err != nil {
		return err
	}

	if s.privKeyID != "" {
		return s.uploadSignedRelease(ctx, m, file)
	}

	ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()

	pb := newBarProgress(file)
	defer func() { _ = pb.Close() }()

	return s.m.Upload(ctx, m, pb)
}
