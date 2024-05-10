package secret

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ernestrc/sensible/pager"
	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cli/format"
	"github.com/unstablebuild/blue/iterator"
)

const (
	accessTimeout = 10 * time.Second
)

type secretAccess struct {
	manager  *secretmanager.Service
	fs       *cli.FlagSet
	all      bool
	noPrompt bool
}

func newSecretAccessCLI(s *secretmanager.Service) cli.CLI {
	c := &secretAccess{
		manager: s,
	}
	c.fs = cli.NewFlagSet("access")
	c.fs.BoolVar(&c.all, "A", false, "Include all enabled secret versions, rather than the latest.")
	c.fs.BoolVar(&c.noPrompt, "y", false, "Do not prompt user, simply print the secret to stdout.")
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

	if s.noPrompt {
		for _, sec := range res {
			fmt.Fprintf(os.Stdout, string(sec.Payload))
		}
		return nil
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
