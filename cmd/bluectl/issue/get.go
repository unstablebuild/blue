// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
