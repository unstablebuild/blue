package release

import (
	"context"
	"fmt"
	"io/ioutil"
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
		Summary:  "Describe a package bundle",
		Synopsis: "<package> <version>",
		Options:  *s.fs,
	}
}

func printableBundle(man release.Bundle) (string, error) {
	var m manifest
	m.fromModel(man)
	return m.toYAML()
}

func (s releaseDescribe) Run(ctx context.Context, args []string) error {
	args, ok, err := cli.ParseUsage(s, s.fs, 2, args)
	if err != nil || !ok {
		return err
	}

	pack := args[0]
	version := release.Version(args[1])

	ctx, cancel := context.WithTimeout(ctx, defaultDescribeTimeout)
	defer cancel()

	man, err := s.m.Get(ctx, pack, version,
		release.NopProgressWriter(ioutil.Discard))
	if err != nil {
		return err
	}

	data, err := printableBundle(man)
	if err != nil {
		return err
	}
	fmt.Printf("\n---\n%s\n", data)

	return nil
}
