package panicreport

import (
	"context"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
)

const (
	deleteTimeout = 30 * time.Second
)

type panicReportDelete struct {
	m  release.Manager
	fs *cli.FlagSet
}

func newPanicReportDeleteCLI(m release.Manager) cli.CLI {
	return panicReportDelete{
		m:  m,
		fs: cli.NewFlagSet("delete"),
	}
}

func (s panicReportDelete) Man() cli.Manual {
	return cli.Manual{
		Name:     "delete",
		Summary:  "Delete a panic report",
		Synopsis: "<uuid>",
		Options:  *s.fs,
	}
}

func (s panicReportDelete) Run(ctx context.Context, args []string) error {
	args, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), deleteTimeout)
	defer cancel()

	return s.m.DeletePanicReport(ctx, args[0])
}
