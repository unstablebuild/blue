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
	"os"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/issue"
	"gopkg.in/yaml.v3"
)

type issueCreate struct {
	t        issue.Tracker
	fs       *cli.FlagSet
	filePath string
	author   string
}

func newReportCreateCLI(t issue.Tracker, author string) cli.CLI {
	c := &issueCreate{
		t:      t,
		author: author,
	}
	c.fs = cli.NewFlagSet("create")
	c.fs.StringVar(&c.filePath, "f", "", "Create an issue from a file report.")
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

	r, err := tempIssue(template)
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
