package report

import (
	"context"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/issue"
)

var (
	actionPanic  string = "panic"
	actionDelete string = "delete"
	actionGet    string = "get"
	actionList   string = "list"
	actionClose  string = "close"
)

type reportCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new report cli.CLI and
// initializes it with the given release.Manager.
func NewCLI(t issue.Tracker, version string) cli.CLI {
	return &reportCLI{
		cmds: map[string]cli.CLI{
			actionPanic:  newPanicReportPanicCLI(version, t),
			actionClose:  newPanicReportCloseCLI(t),
			actionDelete: newPanicReportDeleteCLI(t),
			actionGet:    newPanicReportGetCLI(t),
			actionList:   newPanicReportListCLI(t),
		},
		fs: cli.NewFlagSet("report"),
	}
}

func (s *reportCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "report",
		Summary:  "Manage blue's issue tracker",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *reportCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
