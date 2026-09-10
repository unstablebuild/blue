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

package contributor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/contributor"
)

type verifyCLI struct {
	out io.Writer
	fs  *cli.FlagSet
}

func newVerifyCLI() cli.CLI {
	command := &verifyCLI{out: os.Stdout}
	command.fs = cli.NewFlagSet(actionVerify)
	return command
}

func (v *verifyCLI) Man() cli.Manual {
	return cli.Manual{
		Name: actionVerify,
		Summary: "Recompute a contributor round from an exported " +
			"statement (JSON with Program and Round) and report whether " +
			"the published allocation is reproducible.",
		Synopsis: "<statement.json>",
		Options:  *v.fs,
	}
}

func (v *verifyCLI) Run(ctx context.Context, args []string) error {
	_, rest, ok, err := cli.ParseUsage(v, v.fs, 0, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) != 1 {
		return cli.ErrInvalidArgs
	}

	raw, err := os.ReadFile(rest[0])
	if err != nil {
		return err
	}

	var statement contributor.Statement
	if err := json.Unmarshal(raw, &statement); err != nil {
		return fmt.Errorf("parse statement: %v", err)
	}

	if err := contributor.VerifyRound(statement); err != nil {
		return fmt.Errorf("verification failed: %v", err)
	}
	round := statement.Round
	_, _ = fmt.Fprintf(v.out,
		"round %s verified: pool %d cents over %d credits across %d "+
			"entitlements reproduces the published allocation\n",
		round.Month, round.PoolCents, round.TotalCredits,
		len(round.Entitlements))
	return nil
}
