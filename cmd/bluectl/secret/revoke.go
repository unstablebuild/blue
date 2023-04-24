package secret

import (
	"context"
	"time"

	"github.com/ernestrc/blue/auth"
	"github.com/ernestrc/blue/cli"
)

const (
	revokeTimeout = 10 * time.Second
)

type secretRevoke struct {
	store *auth.Store
	fs    *cli.FlagSet
}

func newSecretRevokeCLI(s *auth.Store) cli.CLI {
	return secretRevoke{
		store: s,
		fs:    cli.NewFlagSet("revoke"),
	}
}

func (s secretRevoke) Man() cli.Manual {
	return cli.Manual{
		Name:     "revoke",
		Summary:  "Revoke a secret",
		Synopsis: "<id>",
		Options:  *s.fs,
	}
}

func (s secretRevoke) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), revokeTimeout)
	defer cancel()

	return s.store.Revoke(ctx, args[0])
}
