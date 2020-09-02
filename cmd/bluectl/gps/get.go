package gps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/gps"
)

var defaultGetTimeout = 10 * time.Second

type getCLI struct {
	gpsServerHostname *string
	fs                *cli.FlagSet
}

func newGetCLI(gpsServerHostname *string) cli.CLI {
	return getCLI{
		gpsServerHostname: gpsServerHostname,
		fs:                cli.NewFlagSet("get"),
	}
}

func (c getCLI) Man() cli.Manual {
	return cli.Manual{
		Name:     "get",
		Summary:  "Get a device's position",
		Synopsis: "",
		Options:  *c.fs,
	}
}

func (c getCLI) get(ctx context.Context, deviceID string) (gps.Coordinates, error) {
	hostname := strings.TrimSuffix(*c.gpsServerHostname, "/")
	url := fmt.Sprintf("%s/location/devices/%s", hostname, deviceID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return gps.Coordinates{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return gps.Coordinates{}, err
	}
	defer res.Body.Close()

	d := json.NewDecoder(res.Body)

	var pos gps.Coordinates
	err = d.Decode(&pos)
	if err != nil {
		return gps.Coordinates{}, err
	}

	return pos, nil
}

func (c getCLI) Run(ctx context.Context, args []string) error {
	parsed, _, err := cli.Parse(c.fs, 1, args)
	if err != nil {
		if err == cli.ErrHelp || err == cli.ErrInvalidArgs {
			cli.Usage(c)
			err = nil
		}
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultGetTimeout)
	defer cancel()

	pos, err := c.get(ctx, parsed[0])
	if err != nil {
		return err
	}

	renderTable([]gps.Coordinates{pos})
	return nil
}
