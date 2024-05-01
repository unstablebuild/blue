package gps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/gps"
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

	res, err := http.DefaultClient.Get(url)
	if err != nil {
		return gps.Coordinates{}, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return gps.Coordinates{}, errors.New("device not found")
	default:
		return gps.Coordinates{},
			fmt.Errorf("non-ok http status code: %v", res.Status)
	}

	d := json.NewDecoder(res.Body)

	var pos gps.Coordinates
	err = d.Decode(&pos)
	if err != nil {
		return gps.Coordinates{}, err
	}

	return pos, nil
}

func (c getCLI) Run(ctx context.Context, args []string) error {
	parsed, _, ok, err := cli.ParseUsage(c, c.fs, 1, args)
	if err != nil || !ok {
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
