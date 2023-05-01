package secret

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ernestrc/blue/auth/secretmanager"
	"github.com/ernestrc/blue/cli"
)

const (
	rotateTimeout = 10 * time.Second
)

type secretRotate struct {
	manager         *secretmanager.Service
	fs              *cli.FlagSet
	disablePrevious bool
}

func newSecretRotateCLI(s *secretmanager.Service) cli.CLI {
	ret := &secretRotate{
		manager: s,
	}
	ret.fs = cli.NewFlagSet("rotate")
	ret.fs.BoolVar(&ret.disablePrevious, "D", false, "Disable all previously enabled versions.")
	return ret
}

func (s *secretRotate) Man() cli.Manual {
	return cli.Manual{
		Name:     "rotate",
		Summary:  "Rotate a secret by uploading a new verions with a new payload.",
		Synopsis: "[options] <id> <filename>",
		Options:  *s.fs,
	}
}

func (s *secretRotate) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 2, args)
	if err != nil || !ok {
		return err
	}
	secretID := args[0]
	filename := args[1]

	payload, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read file: %v", err)
	}

	if len(payload) == 0 {
		return errors.New("empty payload file")
	}

	// remove last EOL
	if payload[len(payload)-1] == '\n' {
		payload = payload[:len(payload)-1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), rotateTimeout)
	defer cancel()

	disabled, err := s.manager.RotateSecret(ctx, secretID, payload, s.disablePrevious)
	if err != nil {
		return err
	}

	if s.disablePrevious {
		fmt.Fprintf(os.Stdout, "disabled %d versions", disabled)
	}

	return nil
}
