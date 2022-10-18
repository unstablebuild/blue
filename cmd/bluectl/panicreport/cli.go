package panicreport

import (
	"context"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
)

var (
	actionPanic  string = "panic"
	actionDelete string = "delete"
	actionGet    string = "get"
	actionList   string = "list"
)

type panicreportCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new panicreport cli.CLI and
// initializes it with the given release.Manager.
func NewCLI(m release.Manager, version string) cli.CLI {
	return &panicreportCLI{
		cmds: map[string]cli.CLI{
			actionPanic:  newPanicReportPanicCLI(version, m),
			actionDelete: newPanicReportDeleteCLI(m),
			actionGet:    newPanicReportGetCLI(m),
			actionList:   newPanicReportListCLI(m),
		},
		fs: cli.NewFlagSet("panicreport"),
	}
}

func (s *panicreportCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "panicreport",
		Summary:  "Manage blue panic reports",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *panicreportCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
