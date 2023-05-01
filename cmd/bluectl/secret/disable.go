package secret

import (
	"context"
	"time"

	"github.com/ernestrc/blue/auth/secretmanager"
	"github.com/ernestrc/blue/cli"
)

const (
	disableTimeout = 10 * time.Second
)

type secretDisable struct {
	manager *secretmanager.Service
	fs      *cli.FlagSet
}

func newSecretDisableCLI(s *secretmanager.Service) cli.CLI {
	ret := &secretDisable{
		manager: s,
	}
	ret.fs = cli.NewFlagSet("disable")
	return ret
}

func (s *secretDisable) Man() cli.Manual {
	return cli.Manual{
		Name:     "disable",
		Summary:  "Disable a secret version.",
		Synopsis: "[options] <id> <version>",
		Options:  *s.fs,
	}
}

func (s *secretDisable) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 2, args)
	if err != nil || !ok {
		return err
	}
	secretID := args[0]
	versionID := args[1]

	ctx, cancel := context.WithTimeout(context.Background(), disableTimeout)
	defer cancel()

	return s.manager.DisableSecretVersion(ctx, secretID, versionID)
}
