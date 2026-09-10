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
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/contributor"
)

func testStatement(t *testing.T) contributor.Statement {
	program := contributor.Program{
		Version:              "2026-01",
		Currency:             "usd",
		PoolBps:              2000,
		CoveredReceipts:      "all subscription revenue",
		CreditDurationRounds: 24,
		PaymentTimetable:     "30 days after quarter end",
		PayoutDueDays:        30,
		CalculationVersion:   contributor.CalculationVersion,
		EffectiveFrom:        "2026-01",
	}
	receipts := []contributor.Receipt{{
		Source: "inv-1", Month: "2026-03", GrossCents: 140000,
		Currency: "usd",
	}}
	awards := []contributor.Award{{
		Splits: []contributor.Split{
			{Account: "a", Credits: 60},
			{Account: "b", Credits: 40},
		},
		Activation:    "2026-01",
		Expiration:    "2028-01",
		Evidence:      []string{"https://github.com/unstablebuild/blue/pull/1"},
		PolicyVersion: "2026-01",
		Status:        contributor.AwardApproved,
	}}
	round, _, err := contributor.ComputeRound(
		program, "2026-03", receipts, awards, 0)
	require.NoError(t, err)
	return contributor.Statement{Program: program, Round: round}
}

func writeStatement(t *testing.T, s contributor.Statement) string {
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "statement.json")
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	return path
}

func TestVerifyCLI(t *testing.T) {
	ctx := context.Background()

	t.Run("verifies valid statement", func(t *testing.T) {
		path := writeStatement(t, testStatement(t))
		out := new(bytes.Buffer)
		cli := newVerifyCLI().(*verifyCLI)
		cli.out = out
		require.NoError(t, cli.Run(ctx, []string{path}))
		assert.Contains(t, out.String(), "verified")
	})

	t.Run("rejects tampered statement", func(t *testing.T) {
		s := testStatement(t)
		s.Round.Entitlements[0].AmountCents++
		path := writeStatement(t, s)
		out := new(bytes.Buffer)
		cli := newVerifyCLI().(*verifyCLI)
		cli.out = out
		err := cli.Run(ctx, []string{path})
		assert.ErrorContains(t, err, "verification failed")
	})

	t.Run("rejects malformed input", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.json")
		require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))
		cli := newVerifyCLI().(*verifyCLI)
		cli.out = new(bytes.Buffer)
		err := cli.Run(ctx, []string{path})
		assert.ErrorContains(t, err, "parse statement")
	})
}
