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
	"os"
	"strings"
	"time"

	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cli/cliformat"
	"github.com/unstablebuild/blue/iterator"
)

const (
	describeTimeout = 10 * time.Second
)

type secretDescribe struct {
	manager *secretmanager.Service
	fs      *cli.FlagSet
	format  string
}

func newSecretDescribeCLI(s *secretmanager.Service) cli.CLI {
	c := &secretDescribe{
		manager: s,
	}
	c.fs = cli.NewFlagSet("describe")
	c.fs.StringVar(&c.format, "F", "table", "Choose output format. Options: 'table', 'json' or a Go text/template.")
	return c
}

func (s *secretDescribe) Man() cli.Manual {
	return cli.Manual{
		Name:     "describe",
		Summary:  "Describe a secret and its metadata.",
		Synopsis: "[options] <id>",
		Options:  *s.fs,
	}
}

func (s *secretDescribe) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	id := args[0]

	ctx, cancel := context.WithTimeout(ctx, describeTimeout)
	defer cancel()

	secView, err := s.manager.GetSecret(ctx, id)
	if err != nil {
		return err
	}

	sec := iterator.FromSlice([]secretmanager.Secret{secView})
	switch strings.ToLower(s.format) {
	case "json":
		t := cliformat.JSON[secretmanager.Secret]()
		return t.Format(os.Stdout, sec)
	case "table":
		t := cliformat.Table[secretmanager.Secret]([]string{"ID", "CreatedAt", "Annotations"})
		return t.Format(os.Stdout, sec)
	default:
		t, err := cliformat.Template[secretmanager.Secret](s.format)
		if err == nil {
			return t.Format(os.Stdout, sec)
		}
		return cli.ErrInvalidArgs
	}
}
