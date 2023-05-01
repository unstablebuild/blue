package secret

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/ernestrc/blue/auth/secretmanager"
	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/cli/format"
	"github.com/ernestrc/blue/iterator"
)

const (
	describeTimeout = 10 * time.Second
)

type secretDescribe struct {
	manager *secretmanager.Service
	fs      *cli.FlagSet
	format  string
}

func newSecretDescribeCLI(s *secretmanager.Service) cli.CLI {
	c := &secretDescribe{
		manager: s,
	}
	c.fs = cli.NewFlagSet("describe")
	c.fs.StringVar(&c.format, "F", "table", "Choose output format. Options: 'table', 'json' or a Go text/template.")
	return c
}

func (s *secretDescribe) Man() cli.Manual {
	return cli.Manual{
		Name:     "describe",
		Summary:  "Describe a secret and its metadata.",
		Synopsis: "[options] <id>",
		Options:  *s.fs,
	}
}

func (s *secretDescribe) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	id := args[0]

	ctx, cancel := context.WithTimeout(ctx, describeTimeout)
	defer cancel()

	secView, err := s.manager.GetSecret(ctx, id)
	if err != nil {
		return err
	}

	sec := iterator.FromSlice([]secretmanager.Secret{secView})
	switch strings.ToLower(s.format) {
	case "json":
		t := format.JSON[secretmanager.Secret]()
		return t.Format(os.Stdout, sec)
	case "table":
		t := format.Table[secretmanager.Secret]([]string{"ID", "CreatedAt", "Annotations"})
		return t.Format(os.Stdout, sec)
	default:
		t, err := format.Template[secretmanager.Secret](s.format)
		if err == nil {
			return t.Format(os.Stdout, sec)
		}
		return cli.ErrInvalidArgs
	}
}
