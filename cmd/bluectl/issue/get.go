package issue

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/issue"
	"github.com/ernestrc/sensible/pager"
	"gopkg.in/yaml.v3"
)

const (
	defaultGetTimeout = 10 * time.Minute
)

type reportGet struct {
	t  issue.Tracker
	fs *cli.FlagSet
}

func newReportGetCLI(t issue.Tracker) cli.CLI {
	return reportGet{
		t:  t,
		fs: cli.NewFlagSet("get"),
	}
}

func (s reportGet) Man() cli.Manual {
	return cli.Manual{
		Name:     "get",
		Summary:  "Get an issue report from the tracker",
		Synopsis: "<id>",
		Options:  *s.fs,
	}
}

func printableReport(r issue.Report) (string, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	err := encoder.Encode(r)
	if err != nil {
		return "", fmt.Errorf("yaml.Encode: %s", err)
	}
	return buf.String(), nil
}

func (s reportGet) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 1, args)
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

	data, err := printableReport(report)
	if err != nil {
		return err
	}

	return pager.PageReader(strings.NewReader(data))
}
