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

package issue

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cli/cliformat"
	"github.com/unstablebuild/blue/issue"
	"github.com/unstablebuild/blue/iterator"
)

const (
	defaultListTimeout = 30 * time.Second
	dateTimeFormat     = "2006-01-02T15:04:05.999"
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

type reportList struct {
	t       issue.Tracker
	fs      *cli.FlagSet
	filters metaFilters
	format  string
}

func newReportListCLI(t issue.Tracker) cli.CLI {
	l := &reportList{
		t:       t,
		filters: metaFilters(map[string]string{}),
	}
	l.fs = cli.NewFlagSet("list")
	l.fs.Var(&l.filters, "f", "Add metadata filter with format 'key=value'")
	l.fs.StringVar(&l.format, "F", "table", "Choose output format. Options: 'table', 'json' or a Go text/template.")
	return l
}

func (s *reportList) Man() cli.Manual {
	return cli.Manual{
		Name:     "list",
		Summary:  "Print all issue reports of a package and optionally version to stdout",
		Options:  *s.fs,
		Synopsis: "[options] <package> [<version>]",
	}
}

func getReportClosedAt(report issue.Report) string {
	if report.ClosedAt.IsZero() {
		return ""
	}
	return report.ClosedAt.Format(dateTimeFormat)
}

func (s *reportList) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, defaultListTimeout)
	defer cancel()

	var ver string
	pkg := args[0]
	if len(args) > 1 {
		ver = args[1]
	}

	log.Debugf("listing reports of package %q version %q with metadata filters: %v",
		pkg, ver, s.filters)

	var reports iterator.Iterator[issue.Report]
	if ver == "" {
		reports, err = s.t.ListPackageReports(ctx, pkg, map[string]string(s.filters))
		if err != nil {
			return err
		}
	} else {
		reports, err = s.t.ListVersionReports(ctx, pkg, ver, map[string]string(s.filters))
		if err != nil {
			return err
		}
	}

	log.Debugf("received reports: %#v", reports)

	reports = &filterDuplicatesIterator{it: reports, seen: make(map[string]struct{})}
	defer func() { _ = reports.Close() }()

	switch strings.ToLower(s.format) {
	case "json":
		t := cliformat.JSON[issue.Report]()
		return t.Format(ctx, os.Stdout, reports)
	case "table":
		type outIssue struct {
			ID        string
			Subject   string
			Author    string
			Labels    string
			CreatedAt string
			ClosedAt  string
		}

		table := cliformat.Table[outIssue]([]string{"ID", "Subject", "Author", "Labels", "CreatedAt", "ClosedAt"})
		return table.Format(ctx, os.Stdout, iterator.Map(reports,
			func(report issue.Report) outIssue {
				var labels []string
				for k := range report.Metadata {
					if !issue.IsInternalLabel(k) {
						labels = append(labels, k)
					}
				}
				return outIssue{
					ID:        report.Metadata[issue.ReportMetadataIDField],
					Subject:   fmt.Sprintf("%10s", report.Subject),
					Author:    report.Author,
					Labels:    strings.Join(labels, ", "),
					CreatedAt: report.CreatedAt.Format(dateTimeFormat),
					ClosedAt:  getReportClosedAt(report),
				}
			}))
	default:
		t, err := cliformat.Template[issue.Report](s.format)
		if err == nil {
			return t.Format(ctx, os.Stdout, reports)
		}
		return cli.ErrInvalidArgs
	}

}

type filterDuplicatesIterator struct {
	it iterator.Iterator[issue.Report]
	// workaround for BLUE-2 issue:
	// un-closed .swp files are double listed.
	// The returned iterator has no access to the document's ID
	// which is an implementation detail of the firestore document.Service
	// so this is the only work-around.
	seen map[string]struct{}
}

func (it *filterDuplicatesIterator) Err() error {
	return it.it.Err()
}

func (it *filterDuplicatesIterator) Close() error {
	return it.it.Close()
}

func (it *filterDuplicatesIterator) Next(ctx context.Context) (issue.Report, bool) {
	for {
		next, ok := it.it.Next(ctx)
		if !ok {
			return next, ok
		}
		id := next.Metadata["_id"]
		if _, seen := it.seen[id]; !seen {
			it.seen[id] = struct{}{}
			return next, ok
		}
	}
}
