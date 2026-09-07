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
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/issue"
)

const updateTimeout = 20 * time.Second

type issueEdit struct {
	t      issue.Tracker
	fs     *cli.FlagSet
	author string
}

func newReportEditCLI(t issue.Tracker, author string) cli.CLI {
	c := &issueEdit{
		t:      t,
		author: author,
	}
	c.fs = cli.NewFlagSet("edit")
	return c
}

func (s *issueEdit) Man() cli.Manual {
	return cli.Manual{
		Name:     "edit",
		Summary:  "Edit an issue in the issue tracker",
		Synopsis: "<id>",
		Options:  *s.fs,
	}
}

func (s *issueEdit) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	issueID := args[0]
	report, err := s.t.GetReport(ctx, issueID)
	if err != nil {
		return err
	}
	// these fields should not be able to be edited
	createdAt := report.CreatedAt
	closedAt := report.ClosedAt
	pkg := report.Package

	r, err := tempIssue(report, true)
	if err != nil {
		return err
	}
	r.UpdatedBy = s.getAuthor()
	r.CreatedAt = createdAt
	r.ClosedAt = closedAt
	r.Package = pkg

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	err = s.t.UpdateReport(ctx, issueID, r)
	if err != nil {
		return err
	}

	fmt.Printf("Updated issue %q", issueID)
	return nil
}

func (s *issueEdit) getAuthor() string {
	if s.author != "" {
		return s.author
	}
	return getDefaultAuthor()
}
