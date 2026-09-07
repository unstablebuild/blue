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

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/release"
)

var (
	actionCreate   string = "create"
	actionDelete   string = "delete"
	actionDescribe string = "describe"
	actionList     string = "list"
	actionUpdate   string = "update"
)

type packageCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new package cli.CLI and
// initializes it with the given package.Manager.
func NewCLI(m release.Manager) cli.CLI {
	return &packageCLI{
		cmds: map[string]cli.CLI{
			actionCreate:   newReleaseCreateCLI(m),
			actionDelete:   newReleaseDeleteCLI(m),
			actionDescribe: newReleaseDescribeCLI(m),
			actionList:     newReleaseListCLI(m),
			actionUpdate:   newReleaseUpdateCLI(m),
		},
		fs: cli.NewFlagSet("package"),
	}
}

func (s *packageCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "package",
		Summary:  "Manage blue packages",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *packageCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
