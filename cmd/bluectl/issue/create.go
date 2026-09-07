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

package issue

import (
	"context"
	"fmt"
	"os"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/issue"
	"gopkg.in/yaml.v3"
)

type issueCreate struct {
	t        issue.Tracker
	fs       *cli.FlagSet
	filePath string
	noEdit   bool
	author   string
}

func newReportCreateCLI(t issue.Tracker, author string) cli.CLI {
	c := &issueCreate{
		t:      t,
		author: author,
	}
	c.fs = cli.NewFlagSet("create")
	c.fs.StringVar(&c.filePath, "f", "", "Create an issue from a file report.")
	c.fs.BoolVar(&c.noEdit, "y", false, "Do not prompt user to edit the issue.")
	return c
}

func (s *issueCreate) Man() cli.Manual {
	return cli.Manual{
		Name:     "create",
		Summary:  "Create an issue in the issue tracker",
		Synopsis: "[options]",
		Options:  *s.fs,
	}
}

func (s *issueCreate) getAuthor() string {
	if s.author != "" {
		return s.author
	}
	return getDefaultAuthor()
}

func (s *issueCreate) Run(ctx context.Context, args []string) error {
	_, _, ok, err := cli.ParseUsage(s, s.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	var template issue.Report
	if s.filePath != "" {
		data, err := os.ReadFile(s.filePath)
		if err != nil {
			return fmt.Errorf("read from file report path %q: %w", s.filePath, err)
		}
		if err := yaml.Unmarshal(data, &template); err != nil {
			return fmt.Errorf("unmarshal yaml file report %q: %w", s.filePath, err)
		}
	}
	template.Author = s.getAuthor()

	r, err := tempIssue(template, !s.noEdit)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	id, err := s.t.CreateReport(ctx, r)
	if err != nil {
		return err
	}
	fmt.Printf("Created issue %q", id)
	return nil
}
