package report

import (
	"context"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/issue"
)

const (
	deleteTimeout = 30 * time.Second
)

type reportDelete struct {
	t  issue.Tracker
	fs *cli.FlagSet
}

func newPanicReportDeleteCLI(t issue.Tracker) cli.CLI {
	return reportDelete{
		t:  t,
		fs: cli.NewFlagSet("delete"),
	}
}

func (s reportDelete) Man() cli.Manual {
	return cli.Manual{
		Name:     "delete",
		Summary:  "Delete a report from the issue tracker",
		Synopsis: "<uuid>",
		Options:  *s.fs,
	}
}

func (s reportDelete) Run(ctx context.Context, args []string) error {
	args, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), deleteTimeout)
	defer cancel()

	return s.t.DeleteReport(ctx, args[0])
}
