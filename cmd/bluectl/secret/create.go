package secret

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ernestrc/blue/auth/secretmanager"
	"github.com/ernestrc/blue/cli"
)

const (
	createTimeout = 10 * time.Second
)

type metadataFlag []string

func (i *metadataFlag) String() string {
	return ""
}

func (i *metadataFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type secretCreate struct {
	store     *secretmanager.Service
	fs        *cli.FlagSet
	mdataFlag metadataFlag
}

func newSecretCreateCLI(s *secretmanager.Service) cli.CLI {
	c := &secretCreate{
		store: s,
	}
	c.fs = cli.NewFlagSet("create")
	c.fs.Var(&c.mdataFlag, "d", "Add metadata to the secret. Expects format to be <key>=<value>")
	return c
}

func (s *secretCreate) Man() cli.Manual {
	return cli.Manual{
		Name:     "create",
		Summary:  "Create a secret.",
		Synopsis: "[options] <id>",
		Options:  *s.fs,
	}
}

func (s *secretCreate) parseMetadataFlag() (map[string]string, error) {
	ret := make(map[string]string)
	for _, arg := range s.mdataFlag {
		kv := strings.Split(arg, "=")
		if len(kv) != 2 {
			return nil, errors.New("invalid -d format")
		}
		ret[kv[0]] = kv[1]
	}
	return ret, nil
}

func (s *secretCreate) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	id := args[0]

	mdata, err := s.parseMetadataFlag()
	if err != nil {
		cli.Usage(s)
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	return s.store.CreateSecret(ctx, id, mdata)
}
