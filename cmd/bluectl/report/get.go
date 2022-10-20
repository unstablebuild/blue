package report

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ernestrc/blue/cli"
	"github.com/ernestrc/blue/debug"
	"github.com/ernestrc/blue/release"
	"github.com/ernestrc/sensible/pager"
	"gopkg.in/yaml.v3"
)

const (
	defaultGetTimeout = 10 * time.Minute
)

type reportGet struct {
	t  release.Tracker
	fs *cli.FlagSet
}

func newPanicReportGetCLI(t release.Tracker) cli.CLI {
	return reportGet{
		t:  t,
		fs: cli.NewFlagSet("get"),
	}
}

func (s reportGet) Man() cli.Manual {
	return cli.Manual{
		Name:     "get",
		Summary:  "Get a bug report",
		Synopsis: "<uuid>",
		Options:  *s.fs,
	}
}

func printablePanicReport(r debug.Report) (string, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	err := encoder.Encode(r)
	if err != nil {
		return "", fmt.Errorf("yaml.Encode: %s", err)
	}
	return buf.String(), nil
}

func (s reportGet) Run(ctx context.Context, args []string) error {
	args, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	id := args[0]

	ctx, cancel := context.WithTimeout(ctx, defaultGetTimeout)
	defer cancel()

	report, err := s.t.GetReport(ctx, id)
	if err != nil {
		return err
	}

	data, err := printablePanicReport(report)
	if err != nil {
		return err
	}

	return pager.PageReader(strings.NewReader(data))
}
