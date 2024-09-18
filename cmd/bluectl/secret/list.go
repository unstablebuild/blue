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
)

const (
	defaultListTimeout = 30 * time.Second
)

type secretList struct {
	manager *secretmanager.Service
	fs      *cli.FlagSet
	filters string
	format  string
}

func newSecretListCLI(manager *secretmanager.Service) cli.CLI {
	l := &secretList{
		manager: manager,
	}
	l.fs = cli.NewFlagSet("list")
	l.fs.StringVar(&l.filters, "f", "", "Add filters. See format https://cloud.google.com/secret-manager/docs/filtering.")
	l.fs.StringVar(&l.format, "F", "table", "Choose output format. Options: 'table', 'json' or a Go text/template.")
	return l
}

func (s *secretList) Man() cli.Manual {
	return cli.Manual{
		Name:     "list",
		Summary:  "Print secret views to stdout",
		Options:  *s.fs,
		Synopsis: "[options]",
	}
}

func (s *secretList) Run(ctx context.Context, args []string) error {
	_, _, ok, err := cli.ParseUsage(s, s.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultListTimeout)
	defer cancel()

	packages, err := s.manager.ListSecrets(ctx, s.filters)
	if err != nil {
		return err
	}
	defer packages.Close()

	switch strings.ToLower(s.format) {
	case "json":
		t := cliformat.JSON[secretmanager.Secret]()
		return t.Format(ctx, os.Stdout, packages)
	case "table":
		t := cliformat.Table[secretmanager.Secret]([]string{"ID", "CreatedAt", "Annotations"})
		return t.Format(ctx, os.Stdout, packages)
	default:
		t, err := cliformat.Template[secretmanager.Secret](s.format)
		if err == nil {
			return t.Format(ctx, os.Stdout, packages)
		}
		return cli.ErrInvalidArgs
	}
}
