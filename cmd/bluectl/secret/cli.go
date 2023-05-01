package secret

import (
	"context"

	"github.com/ernestrc/blue/auth/secretmanager"
	"github.com/ernestrc/blue/cli"
)

var (
	actionCreate   string = "create"
	actionRotate   string = "rotate"
	actionAccess   string = "access"
	actionList     string = "list"
	actionDisable  string = "disable"
	actionDescribe string = "describe"
)

type secretCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new secret cli.CLI and
// initializes it with the given auth.SecretStore.
func NewCLI(s *secretmanager.Service) cli.CLI {
	return &secretCLI{
		cmds: map[string]cli.CLI{
			actionCreate:   newSecretCreateCLI(s),
			actionRotate:   newSecretRotateCLI(s),
			actionAccess:   newSecretAccessCLI(s),
			actionList:     newSecretListCLI(s),
			actionDisable:  newSecretDisableCLI(s),
			actionDescribe: newSecretDescribeCLI(s),
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
