package issue

import (
	"context"
	"fmt"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/issue"
)

type issueCreate struct {
	t  issue.Tracker
	fs *cli.FlagSet
}

func newReportCreateCLI(t issue.Tracker) cli.CLI {
	c := &issueCreate{
		t: t,
	}
	c.fs = cli.NewFlagSet("create")
	return c
}

func (s *issueCreate) Man() cli.Manual {
	return cli.Manual{
		Name:     "create",
		Summary:  "Create an issue in the issue tracker",
		Synopsis: "",
		Options:  *s.fs,
	}
}

func (s *issueCreate) Run(ctx context.Context, args []string) error {
	_, ok, err := cli.ParseUsage(s, s.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	template := issue.Report{Author: getDefaultAuthor()}
	r, err := tempIssue(template, getDefaultAuthor())
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
