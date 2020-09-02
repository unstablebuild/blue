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

	res, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-ok http status code: %v", res.Status)
	}

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
	renderTable(poss)
	return nil
}
