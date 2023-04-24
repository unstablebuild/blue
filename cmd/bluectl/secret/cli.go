package secret

import (
	"context"

	"github.com/ernestrc/blue/auth"
	"github.com/ernestrc/blue/cli"
)

var (
	actionCreate   string = "create"
	actionRevoke   string = "revoke"
	actionDescribe string = "describe"
	actionList     string = "list"
)

type secretCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new secret cli.CLI and
// initializes it with the given auth.Store.
func NewCLI(s *auth.Store) cli.CLI {
	return &secretCLI{
		cmds: map[string]cli.CLI{
			actionCreate: newSecretCreateCLI(s),
			actionRevoke: newSecretRevokeCLI(s),
			actionList:   newSecretListCLI(s),
		},
		fs: cli.NewFlagSet("secret"),
	}
}

func (s *secretCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "secret",
		Summary:  "Manage blue secrets",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *secretCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
