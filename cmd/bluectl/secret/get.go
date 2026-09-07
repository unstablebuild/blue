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
		return t.Format(ctx, os.Stdout, sec)
	case "table":
		t := cliformat.Table[secretmanager.Secret](
			[]string{"ID", "CreatedAt", "Annotations"})
		return t.Format(ctx, os.Stdout, sec)
	default:
		t, err := cliformat.Template[secretmanager.Secret](s.format)
		if err == nil {
			return t.Format(ctx, os.Stdout, sec)
		}
		return cli.ErrInvalidArgs
	}
}
