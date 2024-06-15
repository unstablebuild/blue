// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
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

	r, err := tempIssue(report)
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
