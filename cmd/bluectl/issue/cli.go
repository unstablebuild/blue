package issue

import (
	"context"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/issue"
)

var (
	actionPanic  string = "panic"
	actionCreate string = "create"
	actionEdit   string = "edit"
	actionDelete string = "delete"
	actionGet    string = "get"
	actionList   string = "list"
	actionClose  string = "close"
)

type reportCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new issue cli.CLI and
// initializes it with the given release.Manager.
func NewCLI(t issue.Tracker, version string) cli.CLI {
	return &reportCLI{
		cmds: map[string]cli.CLI{
			actionPanic:  newReportPanicCLI(version, t),
			actionCreate: newReportCreateCLI(t),
			actionEdit:   newReportEditCLI(t),
			actionClose:  newReportCloseCLI(t),
			actionDelete: newReportDeleteCLI(t),
			actionGet:    newReportGetCLI(t),
			actionList:   newReportListCLI(t),
		},
		fs: cli.NewFlagSet("issue"),
	}
}

func (s *reportCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "issue",
		Summary:  "Manage blue's issue tracker",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *reportCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
