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

package pack

import (
	"context"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/release"
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
		Summary:  "Delete a package",
		Synopsis: "<package>",
		Options:  *s.fs,
	}
}

func (s releaseDelete) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	return s.m.DeletePackage(ctx, args[0])
}
