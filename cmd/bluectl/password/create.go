package password

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/unstablebuild/blue/auth"
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

type passwordCreate struct {
	store     *auth.PasswordStore
	fs        *cli.FlagSet
	mdataFlag metadataFlag
}

func getDefaultAuthor() string {
	u, err := user.Current()
	if err != nil {
		u = &user.User{Username: "unknown"}
	}
	h, err := os.Hostname()
	if err != nil {
		h = "unknown-host"
	}
	return fmt.Sprintf("%s@%s", u.Username, h)
}

func newPasswordCreateCLI(s *auth.PasswordStore) cli.CLI {
	c := &passwordCreate{
		store: s,
	}
	c.fs = cli.NewFlagSet("create")
	c.fs.Var(&c.mdataFlag, "d", "Add default metadata to manifest. Expects format to be <key>=<value>")
	return c
}

func (s *passwordCreate) Man() cli.Manual {
	return cli.Manual{
		Name:     "create",
		Summary:  "Create a password with an ID from data stored in a file",
		Synopsis: "<id> <datafile>",
		Options:  *s.fs,
	}
}

func (s *passwordCreate) parseMetadataFlag() (map[string]string, error) {
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

func (s *passwordCreate) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 2, args)
	if err != nil || !ok {
		return err
	}
	id := args[0]
	dataFile := args[1]

	mdata, err := s.parseMetadataFlag()
	if err != nil {
		cli.Usage(s)
		return err
	}

	m, err := tempPassword(id, dataFile, getDefaultAuthor(), mdata)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	return s.store.CreatePassword(ctx, m.ID, m.Data, m.Metadata)
}
