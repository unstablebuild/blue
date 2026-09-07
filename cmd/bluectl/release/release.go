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

package release

import (
	"context"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/release"
)

var (
	// rename to create
	actionUpload   string = "upload"
	actionDelete   string = "delete"
	actionGet      string = "get"
	actionDescribe string = "describe"
	actionList     string = "list"
)

type releaseCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new release cli.CLI and
// initializes it with the given release.Manager.
func NewCLI(m release.Manager) cli.CLI {
	return &releaseCLI{
		cmds: map[string]cli.CLI{
			actionUpload:   newReleaseUploadCLI(m),
			actionDelete:   newReleaseDeleteCLI(m),
			actionGet:      newReleaseGetCLI(m),
			actionDescribe: newReleaseDescribeCLI(m),
			actionList:     newReleaseListCLI(m),
		},
		fs: cli.NewFlagSet("release"),
	}
}

func (s *releaseCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "release",
		Summary:  "Manage blue package releases",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *releaseCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
