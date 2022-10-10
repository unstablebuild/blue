package pack

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
	"github.com/olekukonko/tablewriter"
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
		Name:    "list",
		Summary: "Print all packages to stdout",
		Options: *s.fs,
	}
}

func (s *releaseList) Run(ctx context.Context, args []string) error {
	_, ok, err := cli.ParseUsage(s, s.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultListTimeout)
	defer cancel()

	packages, err := s.m.ListPackages(ctx, map[string]string(s.filters))
	if err != nil {
		return err
	}

	if len(packages) == 0 {
		fmt.Print("No packages found\n")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Notes", "Latest", "CreatedAt"})
	for _, pack := range packages {
		table.Append([]string{pack.Name, pack.Notes,
			string(pack.Latest), pack.CreatedAt.String()})
	}
	table.Render()

	return nil
}
