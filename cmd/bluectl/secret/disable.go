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
	"time"

	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/cli"
)

const (
	disableTimeout = 10 * time.Second
)

type secretDisable struct {
	manager *secretmanager.Service
	fs      *cli.FlagSet
}

func newSecretDisableCLI(s *secretmanager.Service) cli.CLI {
	ret := &secretDisable{
		manager: s,
	}
	ret.fs = cli.NewFlagSet("disable")
	return ret
}

func (s *secretDisable) Man() cli.Manual {
	return cli.Manual{
		Name:     "disable",
		Summary:  "Disable a secret version.",
		Synopsis: "[options] <id> <version>",
		Options:  *s.fs,
	}
}

func (s *secretDisable) Run(ctx context.Context, args []string) error {
	args, _, ok, err := cli.ParseUsage(s, s.fs, 2, args)
	if err != nil || !ok {
		return err
	}
	secretID := args[0]
	versionID := args[1]

	ctx, cancel := context.WithTimeout(ctx, disableTimeout)
	defer cancel()

	return s.manager.DisableSecretVersion(ctx, secretID, versionID)
}
