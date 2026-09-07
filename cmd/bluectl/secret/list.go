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
	defer func() { _ = packages.Close() }()

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
