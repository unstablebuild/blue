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

package email

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/emailprovider"
)

type sendCLI struct {
	factory            senderFactory
	fs                 *cli.FlagSet
	variables          templateVariables
	sender             string
	replyTo            string
	unsubscribeGroupID int
}

func newSendCLI(
	factory senderFactory,
	defaultSender, defaultReplyTo string,
	defaultUnsubscribeGroupID int,
) cli.CLI {
	command := &sendCLI{
		factory:            factory,
		sender:             defaultSender,
		replyTo:            defaultReplyTo,
		unsubscribeGroupID: defaultUnsubscribeGroupID,
	}
	command.fs = cli.NewFlagSet(actionSend)
	command.fs.Var(&command.variables, "X", "Set a template variable. Expects key=value and may be repeated.")
	command.fs.StringVar(&command.sender, "S", defaultSender, "Override the configured sender email address or named mailbox.")
	command.fs.StringVar(&command.replyTo, "R", defaultReplyTo, "Override the configured reply-to email address or named mailbox.")
	command.fs.IntVar(&command.unsubscribeGroupID, "U", defaultUnsubscribeGroupID, "Override the configured SendGrid unsubscribe group ID.")
	return command
}

func (s *sendCLI) Man() cli.Manual {
	return cli.Manual{
		Name:     actionSend,
		Summary:  "Render and send one email from a Go HTML template file.",
		Synopsis: "[options] <subject> <recipient> <template-file.tmpl>",
		Options:  *s.fs,
	}
}

func (s *sendCLI) Run(ctx context.Context, args []string) error {
	args, rest, ok, err := cli.ParseUsage(s, s.fs, 3, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) != 0 {
		return cli.ErrInvalidArgs
	}
	subject := args[0]
	if strings.TrimSpace(subject) == "" {
		return errors.New("subject cannot be empty")
	}
	if strings.TrimSpace(s.sender) == "" {
		return errors.New("sender is not configured; set email.sender in the bluectl config or pass -S")
	}
	if s.unsubscribeGroupID < 0 {
		return errors.New("unsubscribe group ID cannot be negative")
	}

	recipient, err := parseAddress(args[1])
	if err != nil {
		return err
	}
	sender, err := parseAddress(s.sender)
	if err != nil {
		return fmt.Errorf("sender: %w", err)
	}
	var replyTo *emailprovider.Address
	if s.replyTo != "" {
		address, err := parseAddress(s.replyTo)
		if err != nil {
			return fmt.Errorf("reply-to: %w", err)
		}
		replyTo = &address
	}

	// Render completely before constructing or invoking a sender. In particular,
	// missing template variables fail before any provider can submit email.
	body, err := render(args[2], recipient, s.variables)
	if err != nil {
		return err
	}
	if s.factory == nil {
		return errors.New("email sender is not configured")
	}
	provider, err := s.factory(sender, replyTo, s.unsubscribeGroupID)
	if err != nil {
		return err
	}
	if provider == nil {
		return errors.New("email sender factory returned nil")
	}
	results, err := provider.Send(ctx, []emailprovider.Message{{
		Recipient: recipient,
		Subject:   subject,
		HTMLBody:  string(body),
	}})
	if err != nil {
		return err
	}
	if len(results) != 1 {
		return fmt.Errorf("email provider returned %d results for one message", len(results))
	}
	result := results[0]
	if result.Err != nil {
		return result.Err
	}
	if result.Status != emailprovider.StatusAccepted {
		return fmt.Errorf("email provider returned status %q", result.Status)
	}

	_, err = fmt.Fprintf(os.Stdout, "email accepted for %s\n", recipient.Email)
	return err
}
