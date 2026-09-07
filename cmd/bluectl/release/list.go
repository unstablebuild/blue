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
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cli/cliformat"
	"github.com/unstablebuild/blue/release"
)

const (
	defaultListTimeout = 30 * time.Second
)

type metaFilters map[string]string

func (i *metaFilters) String() string {
	return fmt.Sprintf("%v", map[string]string(*i))
}

func (i *metaFilters) Set(value string) error {
	split := strings.Split(value, "=")
	if len(split) != 2 {
		return fmt.Errorf("invalid metadata filter: %s: expected format is 'key=value'", value)
	}
	(*i)[split[0]] = split[1]
	return nil
}

type releaseList struct {
	m       release.Manager
	fs      *cli.FlagSet
	filters metaFilters
	format  string
}

func newReleaseListCLI(m release.Manager) cli.CLI {
	l := &releaseList{
		m:       m,
		filters: metaFilters(map[string]string{}),
	}
	l.fs = cli.NewFlagSet("list")
	l.fs.Var(&l.filters, "f", "Add metadata filter with format 'key=value'")
	l.fs.StringVar(&l.format, "F", "table", "Choose output format. Options: 'table', 'json' or a Go text/template.")
	return l
}

func (s *releaseList) Man() cli.Manual {
	return cli.Manual{
		Name:     "list",
		Summary:  "Print a package's release bundles to stdout",
		Synopsis: "[options] <package>",
		Options:  *s.fs,
	}
}

func (s *releaseList) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	pack := args[0]

	log.Debugf("listing package %q releases with metadata filters: %v",
		pack, s.filters)

	ctx, cancel := context.WithTimeout(ctx, defaultListTimeout)
	defer cancel()

	bundles, err := s.m.List(ctx, pack, map[string]string(s.filters))
	if err != nil {
		return err
	}
	defer func() { _ = bundles.Close() }()
	switch strings.ToLower(s.format) {
	case "json":
		t := cliformat.JSON[release.Bundle]()
		return t.Format(ctx, os.Stdout, bundles)
	case "table":
		t := cliformat.Table[release.Bundle]([]string{"Package", "Version", "Notes", "CreatedAt"})
		return t.Format(ctx, os.Stdout, bundles)
	default:
		t, err := cliformat.Template[release.Bundle](s.format)
		if err == nil {
			return t.Format(ctx, os.Stdout, bundles)
		}
		return cli.ErrInvalidArgs
	}

}
