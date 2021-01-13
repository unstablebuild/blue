package release

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/cmd/bluectl/options"
	"github.com/ernestrc/blue/release"
)

const (
	createTimeout = 10 * time.Minute
)

// TODO add checksums
type releaseCreate struct {
	m  release.Manager
	fs *cli.FlagSet

	flagNotes  *string
	flagAuthor *string
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

func newReleaseCreateCLI(m release.Manager) cli.CLI {
	c := releaseCreate{
		m: m,
	}
	c.fs = cli.NewFlagSet("create")
	return c
}

func (s releaseCreate) Man() cli.Manual {
	return cli.Manual{
		Name:     "create",
		Summary:  "Create a release with the given tag and tar file",
		Synopsis: "<tag> <filename>",
		Options:  *s.fs,
	}
}

func (s releaseCreate) Run(ctx context.Context, args []string) error {
	args, _, err := cli.Parse(s.fs, 2, args)
	if err != nil {
		if err == cli.ErrHelp || err == cli.ErrInvalidArgs {
			cli.Usage(s)
			err = nil
		}
		return err
	}

	file, err := options.OpenFile(args[1])
	if err != nil {
		return err
	}
	defer file.Close()

	m, err := tempManifest(args[0])
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	return s.m.Create(ctx, m, file)
}
