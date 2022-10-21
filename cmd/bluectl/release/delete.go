package release

import (
	"context"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
)

const (
	deleteTimeout = 1 * time.Minute
)

type releaseDelete struct {
	m  release.Manager
	fs *cli.FlagSet
}

func newReleaseDeleteCLI(m release.Manager) cli.CLI {
	return releaseDelete{
		m:  m,
		fs: cli.NewFlagSet("delete"),
	}
}

func (s releaseDelete) Man() cli.Manual {
	return cli.Manual{
		Name:     "delete",
		Summary:  "Delete a package bundle",
		Synopsis: "<package> <version>",
		Options:  *s.fs,
	}
}

func (s releaseDelete) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 2, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), deleteTimeout)
	defer cancel()

	return s.m.Delete(ctx, args[0], release.Version(args[1]))
}
