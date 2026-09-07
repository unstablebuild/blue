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

// Package email implements bluectl's email commands.
package email

import (
	"context"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/emailprovider"
)

const (
	actionSend    = "send"
	actionPreview = "preview"
)

type senderFactory func(
	sender emailprovider.Address,
	replyTo *emailprovider.Address,
	unsubscribeGroupID int,
) (emailprovider.Sender, error)

type emailCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// Config contains defaults for bluectl's email commands.
type Config struct {
	Sender             string
	ReplyTo            string
	UnsubscribeGroupID int
}

// NewCLI returns the bluectl email command. The factory may be nil when the
// command is created only to render help.
func NewCLI(factory func(
	emailprovider.Address,
	*emailprovider.Address,
	int,
) (emailprovider.Sender, error), config Config) cli.CLI {
	return &emailCLI{
		cmds: map[string]cli.CLI{
			actionSend: newSendCLI(senderFactory(factory), config.Sender, config.ReplyTo,
				config.UnsubscribeGroupID),
			actionPreview: newPreviewCLI(openPreview),
		},
		fs: cli.NewFlagSet("email"),
	}
}

func (e *emailCLI) Man() cli.Manual {
	commands := make([]cli.Manual, 0, len(e.cmds))
	for _, command := range e.cmds {
		commands = append(commands, command.Man())
	}
	return cli.Manual{
		Name:     "email",
		Summary:  "Render, preview, and send email from Go templates.",
		Synopsis: "<cmd>",
		Commands: commands,
		Options:  *e.fs,
	}
}

func (e *emailCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, e, e.fs, e.cmds, args)
}
