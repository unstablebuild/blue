package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ernestrc/blue/cli"
	releaseCLI "github.com/ernestrc/blue/cmd/bluectl/release"
	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/release"
)

type blueCtl struct {
	cmds  map[string]cli.CLI
	fs    *cli.FlagSet
	db    document.Service
	debug bool
}

func initializeConfig(init initializer, configPath string) (*cliConfig, error) {
	exists, err := init.FolderExists()
	if err != nil {
		return nil, err
	}

	if !exists {
		fmt.Fprint(os.Stdout,
			"It looks like it's the first time using bluectl.\n")
		err := init.Run(context.Background(), nil)
		if err != nil {
			return nil, err
		}
	}

	cfg, err := sourceConfig(configPath)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *blueCtl) initFlagSet() {
	fs := cli.NewFlagSet("blue")
	fs.BoolVar(&c.debug, "d", false, "Run with verbose instrumentation.")

	c.fs = fs
}

// NewBlueCtl returns an blue CLI.
func newBlueCtl(configFolder string) (*blueCtl, error) {
	configFilePath := getCLIConfigFile(configFolder)
	init := newInitializer(configFolder)

	config, err := initializeConfig(init, configFilePath)
	if err != nil {
		return nil, err
	}

	db, err := document.NewFireStore(config.Auth.ProjectID,
		config.Release.Collection, config.Auth.CredentialsFile)
	if err != nil {
		return nil, err
	}

	releaseManager := release.NewDocumentManager(db)

	c := &blueCtl{
		cmds: map[string]cli.CLI{
			"init":    init,
			"release": releaseCLI.NewCLI(releaseManager),
		},
		db: db,
	}

	c.initFlagSet()

	return c, nil
}

func (c *blueCtl) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range c.cmds {
		cmds = append(cmds, cmd.Man())
	}
	return cli.Manual{
		Name:     "blue",
		Summary:  "manage blue related resources",
		Synopsis: "[options] <cmd>",
		Commands: cmds,
		Options:  *c.fs,
	}
}

func (c *blueCtl) Run(ctx context.Context, args []string) error {
	_, rest, err := cli.Parse(c.fs, 0, args)
	if err != nil {
		if err == cli.ErrHelp {
			cli.Usage(c)
			err = nil
		}
		return err
	}

	logging.SetDefaults(c.debug)

	ctx = cli.ContextWithOptions(ctx, c.fs)

	return cli.RunCommand(ctx, rest, c.cmds)
}

func (c *blueCtl) Close() error {
	return c.db.Close()
}
