package pack

import (
	"context"
	"fmt"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
)

const (
	defaultDescribeTimeout = 10 * time.Minute
)

type releaseDescribe struct {
	m  release.Manager
	fs *cli.FlagSet
}

func newReleaseDescribeCLI(m release.Manager) cli.CLI {
	return releaseDescribe{
		m:  m,
		fs: cli.NewFlagSet("describe"),
	}
}

func (s releaseDescribe) Man() cli.Manual {
	return cli.Manual{
		Name:     "describe",
		Summary:  "Describe a package",
		Synopsis: "<package>",
		Options:  *s.fs,
	}
}

func printablePackage(man release.Package) (string, error) {
	var m manifest
	m.fromModel(man)
	return m.toYAML()
}

func (s releaseDescribe) Run(ctx context.Context, args []string) error {
	args, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	pack := args[0]

	ctx, cancel := context.WithTimeout(ctx, defaultDescribeTimeout)
	defer cancel()

	man, err := s.m.GetPackage(ctx, pack)
	if err != nil {
		return err
	}

	data, err := printablePackage(man)
	if err != nil {
		return err
	}
	fmt.Printf("\n---\n%+v\n", data)

	return nil
}
