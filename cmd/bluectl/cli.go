package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ernestrc/blue/auth"
	"github.com/ernestrc/blue/auth/secretmanager"
	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/cmd/bluectl/gps"
	issueCLI "github.com/ernestrc/blue/cmd/bluectl/issue"
	packageCLI "github.com/ernestrc/blue/cmd/bluectl/package"
	passwordCLI "github.com/ernestrc/blue/cmd/bluectl/password"
	releaseCLI "github.com/ernestrc/blue/cmd/bluectl/release"
	secretCLI "github.com/ernestrc/blue/cmd/bluectl/secret"
	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/document/firestore"
	"github.com/ernestrc/blue/issue"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/release"
	multierr "github.com/ernestrc/go-multierror"
)

type blueCtl struct {
	configFolder string
	cmds         map[string]cli.CLI
	fs           *cli.FlagSet
	dbs          []document.Service
	debug        bool
	version      bool
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
	fs.BoolVar(&c.version, "v", false, "Print CLI version information to stdout.")
	fs.BoolVar(&c.debug, "V", false, "Run with verbose instrumentation.")
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
	// add commands if this is just a -h call so we
	// can get their documentation
	if len(c.cmds) == 0 {
		c.cmds = map[string]cli.CLI{
			"init":     newInitializer(c.configFolder),
			"release":  releaseCLI.NewCLI(nil),
			"package":  packageCLI.NewCLI(nil),
			"password": passwordCLI.NewCLI(nil),
			"secret":   secretCLI.NewCLI(nil),
			"gps":      gps.NewCLI(),
			"analysis": newAnalysisCli(),
			"issue":    issueCLI.NewCLI(nil, Tag),
		}
	}
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

	docDB, err := firestore.New(config.Auth.ProjectID,
		config.Release.Collection, config.Auth.CredentialsFile)
	if err != nil {
		return err
	}
	trackerDB, err := firestore.New(config.Auth.ProjectID,
		config.Issue.Collection, config.Auth.CredentialsFile)
	if err != nil {
		return err
	}

	releaseManager := release.NewDocumentManager(docDB)
	issueTracker := issue.NewDocumentTracker(trackerDB)

	passwordDB, err := firestore.New(config.Auth.ProjectID,
		config.Password.Collection, config.Auth.CredentialsFile)
	if err != nil {
		return err
	}
	passwordStore := auth.NewPasswordStore(passwordDB)

	secretManager, err := secretmanager.NewService(config.Auth.ProjectID,
		config.Auth.CredentialsFile)
	if err != nil {
		return err
	}

	c.cmds = map[string]cli.CLI{
		"init":     init,
		"release":  releaseCLI.NewCLI(releaseManager),
		"package":  packageCLI.NewCLI(releaseManager),
		"password": passwordCLI.NewCLI(passwordStore),
		"gps":      gps.NewCLI(),
		"secret":   secretCLI.NewCLI(secretManager),
		"analysis": newAnalysisCli(),
		"issue":    issueCLI.NewCLI(issueTracker, Tag),
	}
	c.dbs = []document.Service{docDB, trackerDB}

	logging.SetDefaults(c.debug)

	return nil
}

func (c *blueCtl) printVersion() {
	fmt.Printf("Bluectl %s\n", Version)
}

func (c *blueCtl) Run(ctx context.Context, args []string) error {
	args, rest, ok, err := cli.ParseUsage(c, c.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	err = c.initializeCli()
	if err != nil {
		return err
	}

	if c.version {
		c.printVersion()
		return nil
	}

	ctx = cli.ContextWithOptions(ctx, c.fs)
	err = cli.RunCommand(ctx, rest, c.cmds)
	switch err {
	case cli.ErrInvalidArgs:
		fmt.Printf("%s\n\n", err)
		fallthrough
	case cli.ErrHelp:
		cli.Usage(c)
		return nil
	default:
		return err
	}
}

func (c *blueCtl) Close() (ret error) {
	for _, db := range c.dbs {
		if err := db.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return ret
}
