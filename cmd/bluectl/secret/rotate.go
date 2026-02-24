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

package secret

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/cli"
)

const (
	rotateTimeout = 10 * time.Second
)

type secretRotate struct {
	manager         *secretmanager.Service
	fs              *cli.FlagSet
	disablePrevious bool
	generate        bool
	generateLen     int
}

func newSecretRotateCLI(s *secretmanager.Service) cli.CLI {
	ret := &secretRotate{
		manager: s,
	}
	ret.fs = cli.NewFlagSet("rotate")
	ret.fs.BoolVar(&ret.disablePrevious, "D", false, "Disable all previously enabled versions.")
	ret.fs.BoolVar(&ret.generate, "g", false, "Generate a secure random secret.")
	ret.fs.IntVar(&ret.generateLen, "s", 64, "Size of the generated secret in bytes if -g is passed.")
	return ret
}

func (s *secretRotate) Man() cli.Manual {
	return cli.Manual{
		Name:     "rotate",
		Summary:  "Rotate a secret by uploading a new verions with a new payload.",
		Synopsis: "[options] <id> [<filename>]",
		Options:  *s.fs,
	}
}

func (s *secretRotate) Run(ctx context.Context, args []string) error {
	args, rest, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	if len(rest) < 1 && !s.generate {
		return cli.ErrInvalidArgs
	}

	secretID := args[0]

	var payload []byte
	if !s.generate {
		filename := rest[0]
		payload, err = os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read file: %v", err)
		}

		if len(payload) == 0 {
			return errors.New("empty payload file")
		}

		// remove last EOL
		if payload[len(payload)-1] == '\n' {
			payload = payload[:len(payload)-1]
		}
	} else {
		payload, err = generateRandomBytes(s.generateLen)
		if err != nil {
			return fmt.Errorf("generate random secret: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, rotateTimeout)
	defer cancel()

	disabled, err := s.manager.RotateSecret(ctx, secretID, payload, s.disablePrevious)
	if err != nil {
		return err
	}

	if s.disablePrevious {
		_, _ = fmt.Fprintf(os.Stdout, "disabled %d versions", disabled)
	}

	return nil
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}
