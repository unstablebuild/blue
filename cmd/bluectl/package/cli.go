package pack

import (
	"context"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
)

var (
	actionCreate   string = "create"
	actionDelete   string = "delete"
	actionDescribe string = "describe"
	actionList     string = "list"
)

type packageCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new package cli.CLI and
// initializes it with the given package.Manager.
func NewCLI(m release.Manager) cli.CLI {
	return &packageCLI{
		cmds: map[string]cli.CLI{
			actionCreate:   newReleaseCreateCLI(m),
			actionDelete:   newReleaseDeleteCLI(m),
			actionDescribe: newReleaseDescribeCLI(m),
			actionList:     newReleaseListCLI(m),
		},
		fs: cli.NewFlagSet("package"),
	}
}

func (s *packageCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "package",
		Summary:  "Manage blue package packages",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *packageCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
