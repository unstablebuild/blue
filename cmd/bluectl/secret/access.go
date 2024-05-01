package secret

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cli/format"
	"github.com/unstablebuild/blue/iterator"
	"github.com/ernestrc/sensible/pager"
)

const (
	accessTimeout = 10 * time.Second
)

type secretAccess struct {
	manager *secretmanager.Service
	fs      *cli.FlagSet
	all     bool
}

func newSecretAccessCLI(s *secretmanager.Service) cli.CLI {
	c := &secretAccess{
		manager: s,
	}
	c.fs = cli.NewFlagSet("access")
	c.fs.BoolVar(&c.all, "A", false, "Include all enabled secret versions, rather than the latest.")
	return c
}

func (s *secretAccess) Man() cli.Manual {
	return cli.Manual{
		Name:     "access",
		Summary:  "Access secret(s) payloads.",
		Synopsis: "[options] <id>",
		Options:  *s.fs,
	}
}

func (s *secretAccess) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	id := args[0]

	ctx, cancel := context.WithTimeout(ctx, accessTimeout)
	defer cancel()

	var res []secretmanager.SecretVersion
	if s.all {
		res, err = s.manager.AccessSecretVersions(ctx, id)
	} else {
		var latest secretmanager.SecretVersion
		latest, err = s.manager.AccessSecretLatest(ctx, id)
		if err == nil {
			res = []secretmanager.SecretVersion{latest}
		}
	}
	if err != nil {
		return err
	}

	// do not show payload in table
	t := format.Table[secretmanager.SecretVersion]([]string{"ID", "Version", "State", "CreatedAt"})
	if err := t.Format(os.Stdout, iterator.FromSlice(res)); err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Do you want view the secret(s) payload now? [y/n]:")
	text, _ := reader.ReadString('\n')
	switch text {
	case "y\n", "Y\n":
		for _, sec := range res {
			err := pager.PageReader(bytes.NewReader(sec.Payload))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
