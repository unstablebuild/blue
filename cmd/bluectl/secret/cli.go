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

package secret

import (
	"context"

	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/cli"
)

var (
	actionCreate   string = "create"
	actionRotate   string = "rotate"
	actionAccess   string = "access"
	actionList     string = "list"
	actionDisable  string = "disable"
	actionDescribe string = "describe"
)

type secretCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI allocatest storage for a new secret cli.CLI and
// initializes it with the given auth.SecretStore.
func NewCLI(s *secretmanager.Service) cli.CLI {
	return &secretCLI{
		cmds: map[string]cli.CLI{
			actionCreate:   newSecretCreateCLI(s),
			actionRotate:   newSecretRotateCLI(s),
			actionAccess:   newSecretAccessCLI(s),
			actionList:     newSecretListCLI(s),
			actionDisable:  newSecretDisableCLI(s),
			actionDescribe: newSecretDescribeCLI(s),
		},
		fs: cli.NewFlagSet("secret"),
	}
}

func (s *secretCLI) Man() cli.Manual {
	var cmds []cli.Manual
	for _, cmd := range s.cmds {
		cmds = append(cmds, cmd.Man())
	}

	return cli.Manual{
		Name:     "secret",
		Summary:  "Manage blue secrets",
		Synopsis: "<cmd>",
		Commands: cmds,
		Options:  *s.fs,
	}
}

func (s *secretCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, s, s.fs, s.cmds, args)
}
