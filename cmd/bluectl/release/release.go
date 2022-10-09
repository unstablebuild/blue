package release

import (
	"context"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/release"
)

var (
	actionUpload   string = "upload"
	actionDelete   string = "delete"
	actionGet      string = "get"
	actionDescribe string = "describe"
	actionList     string = "list"
)

type releaseCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new release cli.CLI and
// initializes it with the given release.Manager.
func NewCLI(m release.Manager) cli.CLI {
	return &releaseCLI{
		cmds: map[string]cli.CLI{
			actionUpload:   newReleaseUploadCLI(m),
			actionDelete:   newReleaseDeleteCLI(m),
			actionGet:      newReleaseGetCLI(m),
			actionDescribe: newReleaseDescribeCLI(m),
			actionList:     newReleaseListCLI(m),
		},
		fs: cli.NewFlagSet("release"),
	}
}

func (s *releaseCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "release",
		Summary:  "Manage blue package releases",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *releaseCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
