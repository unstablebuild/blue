package main

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/ernestrc/blue/cli"
)

const configDirectory = "~/.blue"

func exitWithError(err error) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		exitWithError(err)
	}

	ctl, err := newBlueCtl(path.Join(homeDir, ".blue"))
	if err != nil {
		exitWithError(err)
	}
	defer ctl.Close()

	err = cli.Run(context.Background(), ctl)
	if err != nil {
		if err == cli.ErrInvalidArgs {
			cli.Usage(ctl)
		} else {
			fmt.Fprint(os.Stderr, err)
		}
		os.Exit(1)
	}
}
