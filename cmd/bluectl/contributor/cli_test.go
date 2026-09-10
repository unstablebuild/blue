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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/contributor"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
)

// errUnavailable stands in for the credential or connectivity failure an
// unconfigured machine hits when it tries to open the collections.
var errUnavailable = errors.New("contributor collections unavailable")

func unavailable(context.Context) (Ledger, error) {
	return Ledger{}, errUnavailable
}

// TestVerifyNeedsNoLedger pins the property that makes the audit trail
// credible: a contributor with no access to the project must be able to
// verify a published round. Opening the ledger here is a bug.
func TestVerifyNeedsNoLedger(t *testing.T) {
	path := writeStatement(t, testStatement(t))
	c := NewCLI(unavailable)
	require.NoError(t, c.Run(context.Background(),
		[]string{"verify", path}))
}

func TestLedgerCommandsReportOpenFailure(t *testing.T) {
	ctx := context.Background()
	c := NewCLI(unavailable)
	for _, args := range [][]string{
		{"program", "show"},
		{"award", "list"},
		{"round", "show", "2026-01"},
		{"participant", "list"},
	} {
		assert.ErrorIs(t, c.Run(ctx, args), errUnavailable, args)
	}
}

// testLedger returns an in-memory ledger with a published program and
// one enrolled contributor.
func testLedger(t *testing.T) OpenFunc {
	t.Helper()
	ledger := Ledger{
		Ledger: contributor.Ledger{
			Programs: contributor.NewDocumentProgramStore(
				document.NewInMemoryService()),
			Participants: contributor.NewDocumentParticipantStore(
				document.NewInMemoryService()),
			Awards: contributor.NewDocumentAwardStore(
				document.NewInMemoryService()),
			Receipts: contributor.NewDocumentReceiptStore(
				document.NewInMemoryService()),
			Rounds: contributor.NewDocumentRoundStore(
				document.NewInMemoryService()),
			Obligations: contributor.NewDocumentObligationStore(
				document.NewInMemoryService()),
		},
		Operator: "operator",
	}
	require.NoError(t, ledger.Participants.CreateParticipant(
		context.Background(), contributor.Participant{
			Account:          "auth0|alice",
			GitHubHandle:     "alice-gh",
			AgreementVersion: "2026-01",
			Status:           contributor.ParticipantEligible,
		}))
	return func(context.Context) (Ledger, error) { return ledger, nil }
}

func TestLedgerCommands(t *testing.T) {
	ctx := context.Background()
	open := testLedger(t)
	c := NewCLI(open)

	require.NoError(t, c.Run(ctx, []string{"program", "set",
		"-version", "2026-01",
		"-currency", "USD",
		"-pool-bps", "2000",
		"-covered-receipts", "all subscription revenue",
		"-credit-duration-rounds", "24",
		"-payment-timetable", "30 days after quarter end",
		"-payout-due-days", "30",
		"-effective-from", "2026-01",
	}))

	ledger, err := open(ctx)
	require.NoError(t, err)

	t.Run("program set normalizes and pins the calculation version",
		func(t *testing.T) {
			program, err := ledger.Programs.GetProgram(ctx, "2026-01")
			require.NoError(t, err)
			assert.Equal(t, "usd", program.Currency)
			assert.Equal(t, contributor.CalculationVersion,
				program.CalculationVersion)
		})

	var awardID string
	t.Run("award propose resolves github handles", func(t *testing.T) {
		require.NoError(t, c.Run(ctx, []string{"award", "propose",
			"-activation", "2026-01",
			"-e", "https://github.com/unstablebuild/blue/pull/1",
			"-m", "great work",
			"Alice-GH=60",
		}))
		awards := listAwards(t, ledger, "")
		require.Len(t, awards, 1)
		awardID = awards[0].ID
		assert.Equal(t, []contributor.Split{
			{Account: "auth0|alice", Credits: 60},
		}, awards[0].Splits)
		assert.Equal(t, "operator", awards[0].ProposedBy)
	})

	t.Run("award approve records the operator", func(t *testing.T) {
		require.NoError(t, c.Run(ctx,
			[]string{"award", "approve", "-m", "ship it", awardID}))
		award, err := ledger.Awards.GetAward(ctx, awardID)
		require.NoError(t, err)
		assert.Equal(t, contributor.AwardApproved, award.Status)
		assert.Equal(t, "operator", award.DecidedBy)

		assert.ErrorIs(t,
			c.Run(ctx, []string{"award", "reject", awardID}),
			contributor.ErrAwardAlreadyDecided)
	})

	t.Run("receipt import is idempotent", func(t *testing.T) {
		receipts := `[{"source":"inv-1","month":"2026-01",` +
			`"grossCents":100000,"currency":"USD"}]`
		out := new(bytes.Buffer)
		command := newReceiptImportCLI(open).(*receiptImport)
		command.out = out
		command.in = bytes.NewBufferString(receipts)
		require.NoError(t, command.Run(ctx, []string{}))
		assert.Contains(t, out.String(), "imported 1 receipts, 0 already present")

		receipt, err := ledger.Receipts.GetReceipt(ctx, "inv-1")
		require.NoError(t, err)
		assert.Equal(t, "usd", receipt.Currency)

		out.Reset()
		command.in = bytes.NewBufferString(receipts)
		require.NoError(t, command.Run(ctx, []string{}))
		assert.Contains(t, out.String(), "imported 0 receipts, 1 already present")
	})

	t.Run("round show estimates an open month", func(t *testing.T) {
		out := new(bytes.Buffer)
		command := newRoundShowCLI(open).(*roundShow)
		command.out = out
		require.NoError(t, command.Run(ctx, []string{"2026-01"}))
		assert.Contains(t, out.String(), "round 2026-01 (estimated)")
		assert.Contains(t, out.String(), "auth0|alice")
	})

	t.Run("round close freezes the month", func(t *testing.T) {
		out := new(bytes.Buffer)
		command := newRoundCloseCLI(open).(*roundClose)
		command.out = out
		require.NoError(t, command.Run(ctx, []string{"2026-01"}))

		round, err := ledger.Rounds.GetRound(ctx, "2026-01")
		require.NoError(t, err)
		assert.Equal(t, "operator", round.ClosedBy)
		assert.Equal(t, int64(20000), round.PoolCents)

		assert.ErrorIs(t, command.Run(ctx, []string{"2026-01"}),
			contributor.ErrRoundClosed)
	})
}

func listAwards(
	t *testing.T, ledger Ledger, status contributor.AwardStatus,
) []contributor.Award {
	t.Helper()
	ctx := context.Background()
	it, err := ledger.Awards.ListAwards(ctx, status)
	require.NoError(t, err)
	awards, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	return awards
}
