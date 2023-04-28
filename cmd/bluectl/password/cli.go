package password

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

type passwordCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new password cli.CLI and
// initializes it with the given auth.PasswordStore.
func NewCLI(s *auth.PasswordStore) cli.CLI {
	return &passwordCLI{
		cmds: map[string]cli.CLI{
			actionCreate: newPasswordCreateCLI(s),
			actionRevoke: newPasswordRevokeCLI(s),
			actionList:   newPasswordListCLI(s),
		},
		fs: cli.NewFlagSet("password"),
	}
}

func (s *passwordCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "password",
		Summary:  "Manage blue passwords",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *passwordCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
