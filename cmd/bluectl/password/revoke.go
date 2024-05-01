package password

import (
	"context"
	"time"

	"github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/cli"
)

const (
	revokeTimeout = 10 * time.Second
)

type passwordRevoke struct {
	store *auth.PasswordStore
	fs    *cli.FlagSet
}

func newPasswordRevokeCLI(s *auth.PasswordStore) cli.CLI {
	return passwordRevoke{
		store: s,
		fs:    cli.NewFlagSet("revoke"),
	}
}

func (s passwordRevoke) Man() cli.Manual {
	return cli.Manual{
		Name:     "revoke",
		Summary:  "Revoke a password",
		Synopsis: "<id>",
		Options:  *s.fs,
	}
}

func (s passwordRevoke) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), revokeTimeout)
	defer cancel()

	return s.store.Revoke(ctx, args[0])
}
