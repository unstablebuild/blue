package gps

import (
	"context"

	"github.com/ernestrc/blue/cli"
)

var (
	actionDelete string = "delete"
	actionGet    string = "get"
	actionList   string = "list"
)

type gpsCLI struct {
	cmds              map[string]cli.CLI
	fs                *cli.FlagSet
	gpsServerHostname string
}

// NewCLI allocatest storage for a new gps cli.CLI and
// initializes it with the given http server hostname.
func NewCLI() cli.CLI {
	fs := cli.NewFlagSet("gps")
	c := &gpsCLI{
		fs: fs,
	}
	c.cmds = map[string]cli.CLI{
		// actionDelete: newGPSDeleteCLI(serverHostname),
		"track":   newTrackCLI(&c.gpsServerHostname),
		actionGet: newGetCLI(&c.gpsServerHostname),
		"list":    newGetAllCLI(&c.gpsServerHostname),
	}

	fs.StringVar(&c.gpsServerHostname, "H", "http://127.0.0.1:8080", "GPS server hostname. Must contain uri scheme")

	return c
}

func (s *gpsCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "gps",
		Summary:  "Manage GPS data stored via GPS server",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *gpsCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
