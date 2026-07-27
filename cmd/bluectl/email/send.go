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
	factory   senderFactory
	fs        *cli.FlagSet
	variables templateVariables
	sender    string
	replyTo   string
}

func newSendCLI(factory senderFactory, defaultSender, defaultReplyTo string) cli.CLI {
	command := &sendCLI{
		factory: factory,
		sender:  defaultSender,
		replyTo: defaultReplyTo,
	}
	command.fs = cli.NewFlagSet(actionSend)
	command.fs.Var(&command.variables, "X", "Set a template variable. Expects key=value and may be repeated.")
	command.fs.StringVar(&command.sender, "S", defaultSender, "Override the configured sender email address or named mailbox.")
	command.fs.StringVar(&command.replyTo, "R", defaultReplyTo, "Override the configured reply-to email address or named mailbox.")
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
	provider, err := s.factory(sender, replyTo)
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
