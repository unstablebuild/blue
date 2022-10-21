package release

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
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

type releaseList struct {
	m       release.Manager
	fs      *cli.FlagSet
	filters metaFilters
}

func newReleaseListCLI(m release.Manager) cli.CLI {
	l := &releaseList{
		m:       m,
		filters: metaFilters(map[string]string{}),
	}
	l.fs = cli.NewFlagSet("list")
	l.fs.Var(&l.filters, "f", "Add metadata filter with format 'key=value'")
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

	if len(bundles) == 0 {
		fmt.Printf("No bundles found for package %q\n", pack)
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Package", "Version", "Notes", "CreatedAt"})
	for _, bundle := range bundles {
		table.Append([]string{bundle.Package,
			string(bundle.Version), bundle.Notes,
			bundle.CreatedAt.String()})
	}
	table.Render()

	return nil
}
