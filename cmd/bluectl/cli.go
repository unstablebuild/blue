package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/cmd/bluectl/gps"
	releaseCLI "github.com/ernestrc/blue/cmd/bluectl/release"
	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/release"
)

type blueCtl struct {
	configFolder string
	cmds         map[string]cli.CLI
	fs           *cli.FlagSet
	db           document.Service
	debug        bool
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

func (c *blueCtl) initFlagSet(configFolder string) {
	fs := cli.NewFlagSet("blue")
	fs.BoolVar(&c.debug, "v", false, "Run with verbose instrumentation.")
	fs.StringVar(&c.configFolder, "c", configFolder, "Use a different config folder.")

	c.fs = fs
}

// NewBlueCtl returns an blue CLI.
func newBlueCtl(configFolder string) (*blueCtl, error) {
	c := new(blueCtl)
	c.initFlagSet(configFolder)
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

func (c *blueCtl) initializeCli() error {
	configFilePath := getCLIConfigFile(c.configFolder)
	init := newInitializer(c.configFolder)

	config, err := initializeConfig(init, configFilePath)
	if err != nil {
		return err
	}

	db, err := document.NewFireStore(config.Auth.ProjectID,
		config.Release.Collection, config.Auth.CredentialsFile)
	if err != nil {
		return err
	}

	releaseManager := release.NewDocumentManager(db)

	c.cmds = map[string]cli.CLI{
		"init":    init,
		"release": releaseCLI.NewCLI(releaseManager),
		"gps":     gps.NewCLI(),
	}
	c.db = db

	logging.SetDefaults(c.debug)

	return nil
}

func (c *blueCtl) Run(ctx context.Context, args []string) error {
	_, rest, perr := cli.Parse(c.fs, 0, args)
	if perr != nil && perr != cli.ErrHelp {
		return perr
	}

	err := c.initializeCli()
	if err != nil {
		return err
	}

	if perr == cli.ErrHelp {
		cli.Usage(c)
		return nil
	}

	ctx = cli.ContextWithOptions(ctx, c.fs)
	return cli.RunCommand(ctx, rest, c.cmds)
}

func (c *blueCtl) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}
