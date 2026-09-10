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

// Package contributor implements bluectl's contributor program commands.
package contributor

import (
	"context"

	"github.com/unstablebuild/blue/cli"
)

const actionVerify = "verify"

type contributorCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI returns the bluectl contributor command. It operates on
// exported statements and requires no credentials or configuration.
func NewCLI() cli.CLI {
	return &contributorCLI{
		cmds: map[string]cli.CLI{
			actionVerify: newVerifyCLI(),
		},
		fs: cli.NewFlagSet("contributor"),
	}
}

func (c *contributorCLI) Man() cli.Manual {
	commands := make([]cli.Manual, 0, len(c.cmds))
	for _, command := range c.cmds {
		commands = append(commands, command.Man())
	}
	return cli.Manual{
		Name:     "contributor",
		Summary:  "Audit contributor program allocations.",
		Synopsis: "<cmd>",
		Commands: commands,
		Options:  *c.fs,
	}
}

func (c *contributorCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, c, c.fs, c.cmds, args)
}
