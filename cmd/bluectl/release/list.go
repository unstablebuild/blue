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
