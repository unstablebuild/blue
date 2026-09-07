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
	"context"
	"fmt"
	"io"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/debug"
	"github.com/unstablebuild/blue/issue"
)

const (
	createTimeout = 10 * time.Second
)

type panicReportPanic struct {
	t       issue.Tracker
	fs      *cli.FlagSet
	version string
}

func newReportPanicCLI(_ string, t issue.Tracker) cli.CLI {
	c := &panicReportPanic{
		t: t,
	}
	c.fs = cli.NewFlagSet("panic")
	return c
}

func (s *panicReportPanic) Man() cli.Manual {
	return cli.Manual{
		Name:     "panic",
		Summary:  "Create a bug report by capturing a simulated panic",
		Synopsis: "",
		Options:  *s.fs,
	}
}

func (s *panicReportPanic) Run(ctx context.Context, args []string) error {
	_, _, ok, err := cli.ParseUsage(s, s.fs, 0, args)
	if err != nil || !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	discard := log.New()
	discard.Out = io.Discard

	report, _, ok := debug.CapturePanic(discard, "blue", s.version, func() {
		panic("this is a simulation")
	})
	if ok {
		log.Fatal("expected panic report")
	}
	id, err := s.t.CreateReport(ctx, report)
	if err != nil {
		return err
	}
	fmt.Printf("Created issue %q", id)
	return nil
}
