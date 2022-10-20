package report

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/issue"
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
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

type reportList struct {
	t       issue.Tracker
	fs      *cli.FlagSet
	filters metaFilters
}

func newPanicReportListCLI(t issue.Tracker) cli.CLI {
	l := &reportList{
		t:       t,
		filters: metaFilters(map[string]string{}),
	}
	l.fs = cli.NewFlagSet("list")
	l.fs.Var(&l.filters, "f", "Add metadata filter with format 'key=value'")
	return l
}

func (s *reportList) Man() cli.Manual {
	return cli.Manual{
		Name:     "list",
		Summary:  "Print all issue reports of a package and optionally version to stdout",
		Options:  *s.fs,
		Synopsis: "<package> [<version>]",
	}
}

func (s *reportList) Run(ctx context.Context, args []string) error {
	args, ok, err := cli.ParseUsage(s, s.fs, 1, args)
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

	var reports []issue.Report
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

	if len(reports) == 0 {
		fmt.Print("No reports found\n")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Subject", "Author", "Labels", "CreatedAt"})
	for _, report := range reports {
		var labels []string
		for k := range report.Metadata {
			if !issue.IsInternalLabel(k) {
				labels = append(labels, k)
			}
		}
		table.Append([]string{report.Metadata[issue.ReportMetadataIDField],
			fmt.Sprintf("%10s", report.Subject),
			report.Author,
			strings.Join(labels, ", "),
			report.CreatedAt.String()})
	}
	table.Render()

	return nil
}
