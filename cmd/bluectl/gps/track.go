package gps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/gps"
)

var defaultTrackTimeout = 10 * time.Second

type trackCLI struct {
	gpsServerHostname *string
	fs                *cli.FlagSet
	// fromStr, toStr    string
}

func newTrackCLI(gpsServerHostname *string) cli.CLI {
	fs := cli.NewFlagSet("track")
	cli := trackCLI{
		gpsServerHostname: gpsServerHostname,
		fs:                fs,
	}

	// fs.StringVar(&cli.fromStr, "f", "", "Start of the tracking span")
	// fs.StringVar(&cli.fromStr, "t", "", "End of the tracking span")
	return cli
}

func (c trackCLI) Man() cli.Manual {
	return cli.Manual{
		Name:     "track",
		Summary:  "Track a device's position across a span of time",
		Synopsis: "<device-id>",
		Options:  *c.fs,
	}
}

func (c trackCLI) track(ctx context.Context, deviceID string) (pos []gps.Coordinates, err error) {
	hostname := strings.TrimSuffix(*c.gpsServerHostname, "/")
	url := fmt.Sprintf("%s/track/devices/%s", hostname, deviceID)

	res, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, errors.New("device not found")
	default:
		return nil, fmt.Errorf("non-ok http status code: %v", res.Status)
	}

	d := json.NewDecoder(res.Body)
	err = d.Decode(&pos)
	if err != nil {
		return nil, err
	}

	return pos, nil
}

func (c trackCLI) Run(ctx context.Context, args []string) error {
	parsed, _, err := cli.Parse(c.fs, 1, args)
	if err != nil {
		if err == cli.ErrHelp || err == cli.ErrInvalidArgs {
			cli.Usage(c)
			err = nil
		}
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTrackTimeout)
	defer cancel()

	poss, err := c.track(ctx, parsed[0])
	if err != nil {
		return err
	}

	sort.Slice(poss, func(i, j int) bool {
		return poss[i].UnixTime > poss[j].UnixTime
	})

	renderTable(poss)
	return nil
}
