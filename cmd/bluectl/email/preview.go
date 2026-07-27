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
	"fmt"
	"net/url"
	"os"

	"github.com/ernestrc/sensible/browser"
	"github.com/unstablebuild/blue/cli"
)

type previewOpener func(*url.URL) error

type previewCLI struct {
	fs        *cli.FlagSet
	variables templateVariables
	open      previewOpener
}

func newPreviewCLI(open previewOpener) cli.CLI {
	command := &previewCLI{open: open}
	command.fs = cli.NewFlagSet(actionPreview)
	command.fs.Var(&command.variables, "X", "Set a template variable. Expects key=value and may be repeated.")
	return command
}

func (p *previewCLI) Man() cli.Manual {
	return cli.Manual{
		Name:     actionPreview,
		Summary:  "Render a Go HTML template file and open it in the preferred browser.",
		Synopsis: "[options] <recipient> <template-file.tmpl>",
		Options:  *p.fs,
	}
}

func openPreview(location *url.URL) error {
	preferred, err := browser.FindBrowser()
	if err != nil {
		return err
	}
	return preferred.Start(location)
}

func writePreview(body []byte) (string, error) {
	file, err := os.CreateTemp("", "blue-email-preview-*.html")
	if err != nil {
		return "", fmt.Errorf("create email preview: %w", err)
	}
	path := file.Name()
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write email preview: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close email preview: %w", err)
	}
	return path, nil
}

func (p *previewCLI) Run(_ context.Context, args []string) error {
	args, rest, ok, err := cli.ParseUsage(p, p.fs, 2, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) != 0 {
		return cli.ErrInvalidArgs
	}
	recipient, err := parseAddress(args[0])
	if err != nil {
		return err
	}
	body, err := render(args[1], recipient, p.variables)
	if err != nil {
		return err
	}
	path, err := writePreview(body)
	if err != nil {
		return err
	}
	location := &url.URL{Scheme: "file", Path: path}
	if err := p.open(location); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("open email preview: %w", err)
	}

	_, err = fmt.Fprintf(os.Stdout, "opened email preview %s\n", path)
	return err
}
