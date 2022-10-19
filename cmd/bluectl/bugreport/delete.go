package bugreport

import (
	"context"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
)

const (
	deleteTimeout = 30 * time.Second
)

type bugReportDelete struct {
	m  release.Manager
	fs *cli.FlagSet
}

func newPanicReportDeleteCLI(m release.Manager) cli.CLI {
	return bugReportDelete{
		m:  m,
		fs: cli.NewFlagSet("delete"),
	}
}

func (s bugReportDelete) Man() cli.Manual {
	return cli.Manual{
		Name:     "delete",
		Summary:  "Delete a bug report",
		Synopsis: "<uuid>",
		Options:  *s.fs,
	}
}

func (s bugReportDelete) Run(ctx context.Context, args []string) error {
	args, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), deleteTimeout)
	defer cancel()

	return s.m.DeleteBugReport(ctx, args[0])
}
