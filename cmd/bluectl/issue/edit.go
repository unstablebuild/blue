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
	t  issue.Tracker
	fs *cli.FlagSet
}

func newReportEditCLI(t issue.Tracker) cli.CLI {
	c := &issueEdit{
		t: t,
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

	r, err := tempIssue(report, getDefaultAuthor())
	if err != nil {
		return err
	}
	report.CreatedAt = createdAt
	report.ClosedAt = closedAt
	report.Package = pkg

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	err = s.t.UpdateReport(ctx, issueID, r)
	if err != nil {
		return err
	}

	fmt.Printf("Updated issue %q", issueID)
	return nil
}
