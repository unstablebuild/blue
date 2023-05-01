package secret

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ernestrc/blue/auth/secretmanager"
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

type secretList struct {
	manager *secretmanager.Service
	fs      *cli.FlagSet
	filters string
	format  string
}

func newSecretListCLI(manager *secretmanager.Service) cli.CLI {
	l := &secretList{
		manager: manager,
	}
	l.fs = cli.NewFlagSet("list")
	l.fs.StringVar(&l.filters, "f", "", "Add filters. See format https://cloud.google.com/secret-manager/docs/filtering.")
	l.fs.StringVar(&l.format, "F", "table", "Choose output format. Options: 'table', 'json' or a Go text/template.")
	return l
}

func (s *secretList) Man() cli.Manual {
	return cli.Manual{
		Name:     "list",
		Summary:  "Print secret views to stdout",
		Options:  *s.fs,
		Synopsis: "[options]",
	}
}

func (s *secretList) Run(ctx context.Context, args []string) error {
	_, _, ok, err := cli.ParseUsage(s, s.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultListTimeout)
	defer cancel()

	packages, err := s.manager.ListSecrets(ctx, s.filters)
	if err != nil {
		return err
	}

	switch strings.ToLower(s.format) {
	case "json":
		t := format.JSON[secretmanager.Secret]()
		return t.Format(os.Stdout, packages)
	case "table":
		t := format.Table[secretmanager.Secret]([]string{"ID", "CreatedAt", "Annotations"})
		return t.Format(os.Stdout, packages)
	default:
		t, err := format.Template[secretmanager.Secret](s.format)
		if err == nil {
			return t.Format(os.Stdout, packages)
		}
		return cli.ErrInvalidArgs
	}
}
