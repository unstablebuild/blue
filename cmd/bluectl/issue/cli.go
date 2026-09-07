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

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/issue"
)

var (
	actionPanic  string = "panic"
	actionCreate string = "create"
	actionEdit   string = "edit"
	actionDelete string = "delete"
	actionGet    string = "get"
	actionList   string = "list"
	actionClose  string = "close"
)

type reportCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new issue cli.CLI and
// initializes it with the given release.Manager.
func NewCLI(t issue.Tracker, version string, author string) cli.CLI {
	return &reportCLI{
		cmds: map[string]cli.CLI{
			actionPanic:  newReportPanicCLI(version, t),
			actionCreate: newReportCreateCLI(t, author),
			actionEdit:   newReportEditCLI(t, author),
			actionClose:  newReportCloseCLI(t),
			actionDelete: newReportDeleteCLI(t),
			actionGet:    newReportGetCLI(t),
			actionList:   newReportListCLI(t),
		},
		fs: cli.NewFlagSet("issue"),
	}
}

func (s *reportCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "issue",
		Summary:  "Manage blue's issue tracker",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *reportCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
