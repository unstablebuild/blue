package password

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ernestrc/blue/auth"
	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/cli/format"
)

const (
	defaultListTimeout = 30 * time.Second
)

type metaFilters map[string]string

func (i *metaFilters) String() string {
	return fmt.Sprintf("%v", map[string]string(*i))
}

func (i *metaFilters) Set(value string) error {
	split := strings.Split(value, "=")
	if len(split) != 2 {
		return fmt.Errorf("invalid metadata filter: %s: expected format is 'key=value'", value)
	}
	(*i)[split[0]] = split[1]
	return nil
}

type passwordList struct {
	store   *auth.PasswordStore
	fs      *cli.FlagSet
	filters metaFilters
	format  string
}

func newPasswordListCLI(store *auth.PasswordStore) cli.CLI {
	l := &passwordList{
		store:   store,
		filters: metaFilters(map[string]string{}),
	}
	l.fs = cli.NewFlagSet("list")
	l.fs.Var(&l.filters, "f", "Add metadata filter with format 'key=value'")
	l.fs.StringVar(&l.format, "F", "table", "Choose output format. Options: 'table', 'json' or a Go text/template.")
	return l
}

func (s *passwordList) Man() cli.Manual {
	return cli.Manual{
		Name:     "list",
		Summary:  "Print password views to stdout",
		Options:  *s.fs,
		Synopsis: "[options]",
	}
}

func (s *passwordList) Run(ctx context.Context, args []string) error {
	_, _, ok, err := cli.ParseUsage(s, s.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultListTimeout)
	defer cancel()

	packages, err := s.store.ListPasswords(ctx, map[string]string(s.filters))
	if err != nil {
		return err
	}

	switch strings.ToLower(s.format) {
	case "json":
		t := format.JSON[auth.PasswordView]()
		return t.Format(os.Stdout, packages)
	case "table":
		t := format.Table[auth.PasswordView]([]string{"ID", "CreatedAt", "UpdatedAt", "Revoked", "Metadata"})
		return t.Format(os.Stdout, packages)
	default:
		t, err := format.Template[auth.PasswordView](s.format)
		if err == nil {
			return t.Format(os.Stdout, packages)
		}
		return cli.ErrInvalidArgs
	}
}
