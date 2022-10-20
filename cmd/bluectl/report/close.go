package report

import (
	"context"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/issue"
)

const (
	closeTimeout = 30 * time.Second
)

type reportClose struct {
	t  issue.Tracker
	fs *cli.FlagSet
}

func newReportCloseCLI(t issue.Tracker) cli.CLI {
	return reportClose{
		t:  t,
		fs: cli.NewFlagSet("close"),
	}
}

func (s reportClose) Man() cli.Manual {
	return cli.Manual{
		Name:     "close",
		Summary:  "Close an issue from the issue tracker",
		Synopsis: "<id>",
		Options:  *s.fs,
	}
}

func (s reportClose) Run(ctx context.Context, args []string) error {
	args, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()

	return s.t.CloseReport(ctx, args[0])
}
