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

// Package newsletter implements bluectl's newsletter commands.
package newsletter

import (
	"context"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/document"
)

const actionList = "list"

type newsletterCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI returns the bluectl newsletter command. db is the document service of
// the newsletter subscriber collection and may be nil when the command is
// created only to render help.
func NewCLI(db document.Service) cli.CLI {
	return &newsletterCLI{
		cmds: map[string]cli.CLI{
			actionList: newListCLI(db),
		},
		fs: cli.NewFlagSet("newsletter"),
	}
}

func (n *newsletterCLI) Man() cli.Manual {
	commands := make([]cli.Manual, 0, len(n.cmds))
	for _, command := range n.cmds {
		commands = append(commands, command.Man())
	}
	return cli.Manual{
		Name:     "newsletter",
		Summary:  "Print newsletter subscribers to stdout.",
		Synopsis: "<cmd>",
		Commands: commands,
		Options:  *n.fs,
	}
}

func (n *newsletterCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, n, n.fs, n.cmds, args)
}
