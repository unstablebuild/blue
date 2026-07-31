// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
