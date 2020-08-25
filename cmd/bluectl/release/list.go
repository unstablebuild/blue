package release

import (
	"context"
	"fmt"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
)

const (
	defaultListTimeout = 30 * time.Second
)

type releaseList struct {
	m  release.Manager
	fs *cli.FlagSet
}

func newReleaseListCLI(m release.Manager) cli.CLI {
	return releaseList{
		m:  m,
		fs: cli.NewFlagSet("list"),
	}
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

	ctx, cancel := context.WithTimeout(ctx, defaultListTimeout)
	defer cancel()

	ms, err := s.m.List(ctx)
	if err != nil {
		return err
	}

	for _, manifest := range ms {
		fmt.Printf("%s\n", manifest.ID)
	}

	return nil
}
