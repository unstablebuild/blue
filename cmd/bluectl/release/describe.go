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

package release

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/release"
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
	args, _, ok, err := cli.ParseUsage(s, s.fs, 2, args)
	if err != nil || !ok {
		return err
	}

	pack := args[0]
	version := release.Version(args[1])

	ctx, cancel := context.WithTimeout(ctx, defaultDescribeTimeout)
	defer cancel()

	man, err := s.m.Get(ctx, pack, version,
		release.NopProgressWriter(io.Discard))
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
