package release

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
)

const (
	defaultGetTimeout = 10 * time.Minute
)

type releaseGet struct {
	m  release.Manager
	fs *cli.FlagSet
}

func newReleaseGetCLI(m release.Manager) cli.CLI {
	return releaseGet{
		m:  m,
		fs: cli.NewFlagSet("get"),
	}
}

func (s releaseGet) Man() cli.Manual {
	return cli.Manual{
		Name:     "get",
		Summary:  "Download a release by tag",
		Synopsis: "<tag> <out>",
		Options:  *s.fs,
	}
}

func (s releaseGet) Run(ctx context.Context, args []string) error {
	args, _, err := cli.Parse(s.fs, 2, args)
	if err != nil {
		if err == cli.ErrHelp || err == cli.ErrInvalidArgs {
			cli.Usage(s)
			err = nil
		}
		return err
	}

	id := args[0]
	outfile := args[1]
	f, err := os.Create(outfile)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultGetTimeout)
	defer cancel()

	m, err := s.m.Get(ctx, id, f)
	if err != nil {
		return err
	}
	fmt.Printf("\n---\n%+v\n", m)
	return nil
}
