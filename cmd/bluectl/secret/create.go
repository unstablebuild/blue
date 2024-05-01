package secret

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/cli"
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
	store      *secretmanager.Service
	fs         *cli.FlagSet
	mdataFlag  metadataFlag
	noEncodeID bool
}

func newSecretCreateCLI(s *secretmanager.Service) cli.CLI {
	c := &secretCreate{
		store: s,
	}
	c.fs = cli.NewFlagSet("create")
	c.fs.Var(&c.mdataFlag, "d", "Add metadata to the secret. Expects format to be <key>=<value>")
	c.fs.BoolVar(&c.noEncodeID, "n", false, "Dont't encode passed ID and treat it as the final secret ID.")
	return c
}

func (s *secretCreate) Man() cli.Manual {
	return cli.Manual{
		Name:     "create",
		Summary:  "Create a secret. The final secret ID is printed to stdout.",
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

	if !s.noEncodeID {
		id = auth.EncodeSecretID(id)
	}

	mdata, err := s.parseMetadataFlag()
	if err != nil {
		cli.Usage(s)
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	err = s.store.CreateSecret(ctx, id, mdata)
	if err == nil {
		fmt.Fprint(os.Stdout, id)
		fmt.Fprint(os.Stdout, "\n")
	}
	return err
}
