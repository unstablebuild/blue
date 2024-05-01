package main

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/unstablebuild/blue/cli"
)

var (
	// compile-time variables
	Tag     = "development"
	Commit  = "HEAD"
	Version string
)

func init() {
	Version = fmt.Sprintf("%s (HEAD is %s)", Tag, Commit)
}

func exitWithError(err error) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		exitWithError(err)
	}

	ctl, err := newBlueCtl(path.Join(homeDir, ".bluectl"))
	if err != nil {
		exitWithError(err)
	}
	defer ctl.Close()

	err = cli.Run(context.Background(), ctl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
