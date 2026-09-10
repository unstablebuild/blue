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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enrolledLedger returns a ledger with the test program published and the
// given GitHub handles enrolled as "<handle>-account".
func enrolledLedger(t *testing.T, handles ...string) Ledger {
	t.Helper()
	ctx := context.Background()
	l := newTestLedger()
	require.NoError(t, l.SetProgram(ctx, testProgram()))
	for _, h := range handles {
		require.NoError(t, l.Participants.CreateParticipant(ctx, Participant{
			Account:             h + "-account",
			GitHubHandle:        h,
			AgreementVersion:    "2026-01",
			AgreementAcceptedAt: time.Now().UTC(),
			Status:              ParticipantEligible,
		}))
	}
	return l
}

func testProposal(splits ...HandleSplit) ProposeAwardRequest {
	return ProposeAwardRequest{
		Splits:     splits,
		Activation: "2026-01",
		Evidence:   []string{"https://github.com/unstablebuild/blue/pull/1"},
		ProposedBy: "operator",
	}
}

func TestSetProgram(t *testing.T) {
	ctx := context.Background()

	t.Run("pins the calculation version", func(t *testing.T) {
		l := newTestLedger()
		p := testProgram()
		// A caller cannot publish a policy against another engine.
		p.CalculationVersion = CalculationVersion + 7
		require.NoError(t, l.SetProgram(ctx, p))

		stored, err := l.Programs.GetProgram(ctx, p.Version)
		require.NoError(t, err)
		assert.Equal(t, CalculationVersion, stored.CalculationVersion)
	})

	t.Run("rejects an invalid policy", func(t *testing.T) {
		l := newTestLedger()
		p := testProgram()
		p.PoolBps = 0
		assert.Error(t, l.SetProgram(ctx, p))
	})
}

func TestProposeAward(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves handles case-insensitively", func(t *testing.T) {
		l := enrolledLedger(t, "alice-gh", "bob-gh")
		id, err := l.ProposeAward(ctx, testProposal(
			HandleSplit{GitHubHandle: "Alice-GH", Credits: 60},
			HandleSplit{GitHubHandle: "bob-gh", Credits: 40},
		))
		require.NoError(t, err)

		award, err := l.Awards.GetAward(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, []Split{
			{Account: "alice-gh-account", Credits: 60},
			{Account: "bob-gh-account", Credits: 40},
		}, award.Splits)
		assert.Equal(t, AwardProposed, award.Status)
		assert.Equal(t, "operator", award.ProposedBy)
		assert.False(t, award.ProposedAt.IsZero())
		// Terms are frozen from the policy effective at activation.
		assert.Equal(t, "2026-01", award.PolicyVersion)
		assert.Equal(t, Month("2028-01"), award.Expiration)
	})

	t.Run("unknown handle", func(t *testing.T) {
		l := enrolledLedger(t, "alice-gh")
		_, err := l.ProposeAward(ctx, testProposal(
			HandleSplit{GitHubHandle: "nobody", Credits: 10},
		))
		assert.ErrorIs(t, err, ErrParticipantNotFound)
	})

	t.Run("duplicate recipients", func(t *testing.T) {
		l := enrolledLedger(t, "alice-gh")
		_, err := l.ProposeAward(ctx, testProposal(
			HandleSplit{GitHubHandle: "alice-gh", Credits: 10},
			HandleSplit{GitHubHandle: "ALICE-GH", Credits: 20},
		))
		assert.ErrorContains(t, err, "duplicate recipient")
	})

	t.Run("no program effective in activation month", func(t *testing.T) {
		l := enrolledLedger(t, "alice-gh")
		req := testProposal(HandleSplit{GitHubHandle: "alice-gh", Credits: 10})
		req.Activation = "2025-12"
		_, err := l.ProposeAward(ctx, req)
		assert.ErrorIs(t, err, ErrProgramNotConfigured)
	})

	t.Run("malformed activation month", func(t *testing.T) {
		l := enrolledLedger(t, "alice-gh")
		req := testProposal(HandleSplit{GitHubHandle: "alice-gh", Credits: 10})
		req.Activation = "january"
		_, err := l.ProposeAward(ctx, req)
		assert.Error(t, err)
	})
}

func TestDecideAward(t *testing.T) {
	ctx := context.Background()

	propose := func(t *testing.T) (Ledger, string) {
		t.Helper()
		l := enrolledLedger(t, "alice-gh")
		id, err := l.ProposeAward(ctx, testProposal(
			HandleSplit{GitHubHandle: "alice-gh", Credits: 10}))
		require.NoError(t, err)
		return l, id
	}

	t.Run("approve", func(t *testing.T) {
		l, id := propose(t)
		award, err := l.DecideAward(ctx, id, true, "operator", "great work")
		require.NoError(t, err)
		assert.Equal(t, AwardApproved, award.Status)
		assert.Equal(t, "operator", award.DecidedBy)
		assert.Equal(t, "great work", award.DecisionNote)
		assert.False(t, award.DecidedAt.IsZero())

		stored, err := l.Awards.GetAward(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, AwardApproved, stored.Status)
	})

	t.Run("reject", func(t *testing.T) {
		l, id := propose(t)
		award, err := l.DecideAward(ctx, id, false, "operator", "")
		require.NoError(t, err)
		assert.Equal(t, AwardRejected, award.Status)
	})

	t.Run("decisions are final", func(t *testing.T) {
		l, id := propose(t)
		_, err := l.DecideAward(ctx, id, true, "operator", "")
		require.NoError(t, err)
		_, err = l.DecideAward(ctx, id, false, "operator", "")
		assert.ErrorIs(t, err, ErrAwardAlreadyDecided)
	})

	t.Run("unknown award", func(t *testing.T) {
		l, _ := propose(t)
		_, err := l.DecideAward(ctx, "missing", true, "operator", "")
		assert.ErrorIs(t, err, ErrAwardNotFound)
	})
}

func TestImportReceipts(t *testing.T) {
	ctx := context.Background()
	receipts := []Receipt{
		{Source: "inv-1", Month: "2026-01", GrossCents: 100000, Currency: "usd"},
		{Source: "inv-2", Month: "2026-01", GrossCents: 50000, Currency: "usd"},
	}

	t.Run("re-importing a batch is a no-op", func(t *testing.T) {
		l := newTestLedger()
		imported, duplicates, err := l.ImportReceipts(ctx, receipts)
		require.NoError(t, err)
		assert.Equal(t, 2, imported)
		assert.Equal(t, 0, duplicates)

		imported, duplicates, err = l.ImportReceipts(ctx,
			append(receipts, Receipt{
				Source: "inv-3", Month: "2026-01", GrossCents: 1, Currency: "usd",
			}))
		require.NoError(t, err)
		assert.Equal(t, 1, imported)
		assert.Equal(t, 2, duplicates)
	})

	t.Run("invalid receipt names its source", func(t *testing.T) {
		l := newTestLedger()
		_, _, err := l.ImportReceipts(ctx, []Receipt{
			{Source: "inv-bad", Month: "2026-01", Currency: ""},
		})
		assert.ErrorContains(t, err, "inv-bad")
	})
}
