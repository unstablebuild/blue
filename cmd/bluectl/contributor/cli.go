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
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/contributor"
)

const (
	actionApprove = "approve"
	actionClose   = "close"
	actionImport  = "import"
	actionList    = "list"
	actionPropose = "propose"
	actionReject  = "reject"
	actionSet     = "set"
	actionShow    = "show"
	actionVerify  = "verify"

	defaultTimeout = 30 * time.Second
)

// Ledger is the contributor program ledger together with the identity of
// the operator running the CLI, which is recorded as the proposer,
// decider or closer of everything bluectl writes.
type Ledger struct {
	contributor.Ledger
	Operator string
}

// OpenFunc opens the contributor program collections. Only commands that
// read or write the ledger call it, and only when they run: 'verify'
// audits an exported statement offline and never invokes it, so auditing
// the program requires no credentials and no configuration.
type OpenFunc func(context.Context) (Ledger, error)

// NewCLI returns the bluectl contributor command.
func NewCLI(open OpenFunc) cli.CLI {
	return newGroup("contributor",
		"Administer and audit the contributor program.",
		map[string]cli.CLI{
			"program":     newProgramCLI(open),
			"award":       newAwardCLI(open),
			"receipt":     newReceiptCLI(open),
			"round":       newRoundCLI(open),
			"participant": newParticipantCLI(open),
			actionVerify:  newVerifyCLI(),
		})
}

// group dispatches to a set of sub-commands and carries no behaviour of
// its own.
type group struct {
	name    string
	summary string
	cmds    map[string]cli.CLI
	fs      *cli.FlagSet
}

func newGroup(name, summary string, cmds map[string]cli.CLI) cli.CLI {
	return &group{
		name:    name,
		summary: summary,
		cmds:    cmds,
		fs:      cli.NewFlagSet(name),
	}
}

func (c *group) Man() cli.Manual {
	commands := make([]cli.Manual, 0, len(c.cmds))
	for _, command := range c.cmds {
		commands = append(commands, command.Man())
	}
	return cli.Manual{
		Name:     c.name,
		Summary:  c.summary,
		Synopsis: "<cmd>",
		Commands: commands,
		Options:  *c.fs,
	}
}

func (c *group) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, c, c.fs, c.cmds, args)
}

// parseMonthArg returns the month named by an optional positional
// argument, defaulting to the current month.
func parseMonthArg(args []string) (contributor.Month, error) {
	if len(args) == 0 {
		return contributor.MonthOf(time.Now()), nil
	}
	return contributor.ParseMonth(args[0])
}
