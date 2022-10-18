package panicreport

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

type panicReportGet struct {
	m  release.Manager
	fs *cli.FlagSet
}

func newPanicReportGetCLI(m release.Manager) cli.CLI {
	return panicReportGet{
		m:  m,
		fs: cli.NewFlagSet("get"),
	}
}

func (s panicReportGet) Man() cli.Manual {
	return cli.Manual{
		Name:     "get",
		Summary:  "Get a panic report",
		Synopsis: "<uuid>",
		Options:  *s.fs,
	}
}

func printablePanicReport(r debug.PanicReport) (string, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	err := encoder.Encode(r)
	if err != nil {
		return "", fmt.Errorf("yaml.Encode: %s", err)
	}
	return buf.String(), nil
}

func (s panicReportGet) Run(ctx context.Context, args []string) error {
	args, ok, err := cli.ParseUsage(s, s.fs, 1, args)
	if err != nil || !ok {
		return err
	}

	id := args[0]

	ctx, cancel := context.WithTimeout(ctx, defaultGetTimeout)
	defer cancel()

	report, err := s.m.GetPanicReport(ctx, id)
	if err != nil {
		return err
	}

	data, err := printablePanicReport(report)
	if err != nil {
		return err
	}

	return pager.PageReader(strings.NewReader(data))
}
