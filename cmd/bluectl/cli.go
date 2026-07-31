// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/storage"
	multierr "github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/cli"
	emailCLI "github.com/unstablebuild/blue/cmd/bluectl/email"
	issueCLI "github.com/unstablebuild/blue/cmd/bluectl/issue"
	newsletterCLI "github.com/unstablebuild/blue/cmd/bluectl/newsletter"
	packageCLI "github.com/unstablebuild/blue/cmd/bluectl/package"
	releaseCLI "github.com/unstablebuild/blue/cmd/bluectl/release"
	secretCLI "github.com/unstablebuild/blue/cmd/bluectl/secret"
	"github.com/unstablebuild/blue/document/firestore"
	"github.com/unstablebuild/blue/emailprovider"
	"github.com/unstablebuild/blue/emailprovider/sendgrid"
	"github.com/unstablebuild/blue/issue"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/release/docrelease"
	"github.com/unstablebuild/blue/release/gcsrelease"
)

type blueCtl struct {
	configFolder string
	cmds         map[string]cli.CLI
	fs           *cli.FlagSet
	closers      []io.Closer
	debug        bool
	version      bool
}

func initializeConfig(init initializer, configPath string) (*cliConfig, error) {
	exists, err := init.FolderExists()
	if err != nil {
		return nil, err
	}

	if !exists {
		_, _ = fmt.Fprint(os.Stdout,
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
	fs := cli.NewFlagSet("bluectl")
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
			"init":       newInitializer(c.configFolder),
			"release":    releaseCLI.NewCLI(nil),
			"package":    packageCLI.NewCLI(nil),
			"secret":     secretCLI.NewCLI(nil),
			"email":      emailCLI.NewCLI(nil, emailCLI.Config{}),
			"analysis":   newAnalysisCli(),
			"license":    newLicenseCli(),
			"issue":      issueCLI.NewCLI(nil, Tag, ""),
			"newsletter": newsletterCLI.NewCLI(nil),
		}
	}
	for _, cmd := range c.cmds {
		cmds = append(cmds, cmd.Man())
	}
	return cli.Manual{
		Name:     "bluectl",
		Summary:  "Manage internal resources.",
		Synopsis: "[options] <cmd>",
		Commands: cmds,
		Options:  *c.fs,
	}
}

func (c *blueCtl) newReleaseManager(init initializer, configFilePath string) (*gcsrelease.Manager, error) {
	config, err := initializeConfig(init, configFilePath)
	if err != nil {
		return nil, err
	}
	docDB, err := firestore.New(config.Auth.ProjectID,
		config.Release.Collection, config.Auth.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("firestore: %w", err)
	}
	c.closers = append(c.closers, docDB)

	gcsClient, err := storage.NewClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("gcs: %w", err)
	}
	c.closers = append(c.closers, gcsClient)

	bucket := gcsClient.Bucket(config.Release.Bucket)
	inner := docrelease.NewManager(docDB)
	return gcsrelease.NewManager(inner, gcsrelease.NewBucket(bucket)), nil
}

func (c *blueCtl) initializeCli() error {
	configFilePath := getCLIConfigFile(c.configFolder)
	init := newInitializer(c.configFolder)

	c.cmds = map[string]cli.CLI{
		"init": init,
		"release": cli.Lazy(func(ctx context.Context) (cli.CLI, error) {
			m, err := c.newReleaseManager(init, configFilePath)
			if err != nil {
				return nil, err
			}
			return releaseCLI.NewCLI(m), nil
		}),
		"package": cli.Lazy(func(ctx context.Context) (cli.CLI, error) {
			m, err := c.newReleaseManager(init, configFilePath)
			if err != nil {
				return nil, err
			}
			return packageCLI.NewCLI(m), nil
		}),
		"secret": cli.Lazy(func(ctx context.Context) (cli.CLI, error) {
			config, err := initializeConfig(init, configFilePath)
			if err != nil {
				return nil, err
			}
			secretManager, err := secretmanager.NewService(config.Auth.ProjectID,
				config.Auth.CredentialsFile)
			if err != nil {
				return nil, fmt.Errorf("secretmanager: %w", err)
			}
			c.closers = append(c.closers, secretManager)
			return secretCLI.NewCLI(secretManager), nil
		}),
		"email": cli.Lazy(func(ctx context.Context) (cli.CLI, error) {
			config, err := initializeConfig(init, configFilePath)
			if err != nil {
				return nil, err
			}
			return emailCLI.NewCLI(func(
				sender emailprovider.Address,
				replyTo *emailprovider.Address,
				unsubscribeGroupID int,
			) (emailprovider.Sender, error) {
				return sendgrid.New(sendgrid.Credentials{
					APIKey: config.Email.SendGrid.APIKey,
				}, sendgrid.Config{
					Sender:             sender,
					ReplyTo:            replyTo,
					UnsubscribeGroupID: unsubscribeGroupID,
				})
			}, emailCLI.Config{
				Sender:             config.Email.Sender,
				ReplyTo:            config.Email.ReplyTo,
				UnsubscribeGroupID: config.Email.SendGrid.UnsubscribeGroupID,
			}), nil
		}),
		"analysis": newAnalysisCli(),
		"license":  newLicenseCli(),
		"issue": cli.Lazy(func(ctx context.Context) (cli.CLI, error) {
			config, err := initializeConfig(init, configFilePath)
			if err != nil {
				return nil, err
			}
			trackerDB, err := firestore.New(config.Auth.ProjectID,
				config.Issue.Collection, config.Auth.CredentialsFile)
			if err != nil {
				return nil, fmt.Errorf("firestore: %w", err)
			}
			c.closers = append(c.closers, trackerDB)
			issueTracker := issue.NewDocumentTracker(trackerDB)

			return issueCLI.NewCLI(issueTracker, Tag, config.Issue.Author), nil
		}),
		"newsletter": cli.Lazy(func(ctx context.Context) (cli.CLI, error) {
			config, err := initializeConfig(init, configFilePath)
			if err != nil {
				return nil, err
			}
			subscriberDB, err := firestore.New(config.Auth.ProjectID,
				config.Newsletter.Collection, config.Auth.CredentialsFile)
			if err != nil {
				return nil, fmt.Errorf("firestore: %w", err)
			}
			c.closers = append(c.closers, subscriberDB)
			return newsletterCLI.NewCLI(subscriberDB), nil
		}),
	}

	logging.SetDefaults(c.debug)

	return nil
}

func (c *blueCtl) printVersion() {
	fmt.Printf("bluectl %s\n", Version)
}

func (c *blueCtl) Run(ctx context.Context, args []string) error {
	_, rest, ok, err := cli.ParseUsage(c, c.fs, 0, args)
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
		cli.Usage(c)
		return err
	case cli.ErrHelp:
		cli.Usage(c)
		return nil
	default:
		return err
	}
}

func (c *blueCtl) Close() (ret error) {
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return ret
}
