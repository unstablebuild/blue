package gps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/gps"
	"github.com/olekukonko/tablewriter"
)

var defaultGetAllTimeout = 10 * time.Second

type getAllCLI struct {
	gpsServerHostname *string
	fs                *cli.FlagSet
}

func newGetAllCLI(gpsServerHostname *string) cli.CLI {
	return getAllCLI{
		gpsServerHostname: gpsServerHostname,
		fs:                cli.NewFlagSet("get-all"),
	}
}

func (c getAllCLI) Man() cli.Manual {
	return cli.Manual{
		Name:     "get-all",
		Summary:  "Get all devices positions",
		Synopsis: "",
		Options:  *c.fs,
	}
}

func (c getAllCLI) getAll(ctx context.Context) (pos []gps.Coordinates, err error) {
	hostname := strings.TrimSuffix(*c.gpsServerHostname, "/")
	url := fmt.Sprintf("%s/location/devices", hostname)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	d := json.NewDecoder(res.Body)
	err = d.Decode(&pos)
	if err != nil {
		return nil, err
	}

	return pos, nil
}

func (c getAllCLI) Run(ctx context.Context, args []string) error {
	_, _, err := cli.Parse(c.fs, 0, args)
	if err != nil {
		if err == cli.ErrHelp || err == cli.ErrInvalidArgs {
			cli.Usage(c)
			err = nil
		}
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultGetAllTimeout)
	defer cancel()

	poss, err := c.getAll(ctx)
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Device", "Lat/Lng", "Altitude", "Last Updated"})
	defer table.Render()

	for _, pos := range poss {
		table.Append([]string{
			pos.DeviceID,
			fmt.Sprintf("%f,%f", pos.Latitude, pos.Longitude),
			fmt.Sprintf("%f m", pos.Altitude),
			fmt.Sprintf("%s ago", time.Since(time.Unix(pos.UnixTime, 0)).String()),
		})
	}
	return nil
}
