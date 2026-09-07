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
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/ernestrc/sensible/browser"
	"github.com/unstablebuild/blue/cli"
)

type previewOpener func(*url.URL) error

type previewCLI struct {
	fs          *cli.FlagSet
	variables   templateVariables
	openBrowser bool
	open        previewOpener
	output      io.Writer
}

func newPreviewCLI(open previewOpener) cli.CLI {
	command := &previewCLI{open: open, output: os.Stdout}
	command.fs = cli.NewFlagSet(actionPreview)
	command.fs.Var(&command.variables, "X", "Set a template variable. Expects key=value and may be repeated.")
	command.fs.BoolVar(&command.openBrowser, "o", false,
		"Open the generated HTML using BROWSER, then the first available of open, google-chrome-stable, firefox, or chromium.")
	return command
}

func (p *previewCLI) Man() cli.Manual {
	return cli.Manual{
		Name:     actionPreview,
		Summary:  "Render a Go HTML template file to a temporary HTML file.",
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
	if p.openBrowser {
		location := &url.URL{Scheme: "file", Path: path}
		if err := p.open(location); err != nil {
			_ = os.Remove(path)
			return fmt.Errorf("open email preview: %w", err)
		}
		_, err = fmt.Fprintf(p.output, "opened email preview %s\n", path)
		return err
	}

	_, err = fmt.Fprintf(p.output, "generated email preview %s\n", path)
	return err
}
