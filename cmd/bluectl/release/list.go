package release

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
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
	l := releaseList{
		m:       m,
		filters: metaFilters(map[string]string{}),
	}
	l.fs = cli.NewFlagSet("list")
	l.fs.Var(&l.filters, "f", "Add metadata filter with format 'key=value'")
	return l
}

func (s releaseList) Man() cli.Manual {
	return cli.Manual{
		Name:     "list",
		Summary:  "Print all release tags to stdout",
		Synopsis: "",
		Options:  *s.fs,
	}
}

func (s releaseList) Run(ctx context.Context, args []string) error {
	args, _, err := cli.Parse(s.fs, 0, args)
	if err != nil {
		if err == cli.ErrHelp || err == cli.ErrInvalidArgs {
			cli.Usage(s)
			err = nil
		}
		return err
	}

	log.Debugf("listing releases with metadata filters: %v", s.filters)

	ctx, cancel := context.WithTimeout(ctx, defaultListTimeout)
	defer cancel()

	ms, err := s.m.List(ctx, map[string]string(s.filters))
	if err != nil {
		return err
	}

	for _, manifest := range ms {
		fmt.Printf("%s\n", manifest.ID)
	}

	return nil
}
