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
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
)

func newTestLedger() Ledger {
	return Ledger{
		Programs:     NewDocumentProgramStore(document.NewInMemoryService()),
		Participants: NewDocumentParticipantStore(document.NewInMemoryService()),
		Awards:       NewDocumentAwardStore(document.NewInMemoryService()),
		Receipts:     NewDocumentReceiptStore(document.NewInMemoryService()),
		Rounds:       NewDocumentRoundStore(document.NewInMemoryService()),
		Obligations:  NewDocumentObligationStore(document.NewInMemoryService()),
	}
}

func TestLedger(t *testing.T) {
	ctx := context.Background()
	closedAt := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)

	setup := func(t *testing.T) Ledger {
		l := newTestLedger()
		require.NoError(t, l.Programs.PutProgram(ctx, testProgram()))

		award := approvedAward("2026-01",
			Split{Account: "alice", Credits: 60},
			Split{Account: "bob", Credits: 40})
		_, err := l.Awards.CreateAward(ctx, award)
		require.NoError(t, err)

		require.NoError(t, l.Receipts.CreateReceipt(ctx, Receipt{
			Source: "inv-1", Month: "2026-03", GrossCents: 100000,
			Currency: "usd",
		}))
		return l
	}

	t.Run("estimate does not persist", func(t *testing.T) {
		l := setup(t)
		round, obls, err := l.Estimate(ctx, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, int64(20000), round.PoolCents)
		require.Len(t, obls, 2)
		assert.Equal(t, ObligationEstimated, obls[0].State)

		_, err = l.Rounds.GetRound(ctx, "2026-03")
		assert.Equal(t, document.ErrNotFound, err)
		it, err := l.Obligations.ListAccountObligations(ctx, "alice")
		require.NoError(t, err)
		all, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		assert.Empty(t, all)
	})

	t.Run("close round persists round and obligations", func(t *testing.T) {
		l := setup(t)
		round, err := l.CloseRound(ctx, "2026-03", closedAt, "admin")
		require.NoError(t, err)
		assert.Equal(t, closedAt, round.ClosedAt)
		assert.Equal(t, "admin", round.ClosedBy)

		stored, err := l.Rounds.GetRound(ctx, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, round.PoolCents, stored.PoolCents)
		require.NoError(t, VerifyRound(Statement{
			Program: testProgram(), Round: stored,
		}))

		o, err := l.Obligations.GetObligation(ctx, "2026-03", "alice")
		require.NoError(t, err)
		assert.Equal(t, ObligationEarned, o.State)
		assert.Equal(t, int64(12000), o.AmountCents)
		require.Len(t, o.History, 1)
		assert.Equal(t, ObligationEstimated, o.History[0].From)

		t.Run("closed rounds are immutable", func(t *testing.T) {
			_, err := l.CloseRound(ctx, "2026-03", closedAt, "admin")
			assert.ErrorIs(t, err, ErrRoundClosed)
		})

		t.Run("carryforward flows into next round", func(t *testing.T) {
			// no receipts and no active awards changes in April; pool
			// is zero, carryforward is zero
			next, err := l.CloseRound(ctx, "2026-04",
				closedAt.AddDate(0, 1, 0), "admin")
			require.NoError(t, err)
			assert.Equal(t, int64(0), next.CarryforwardCents)
		})
	})

	t.Run("zero denominator carries forward", func(t *testing.T) {
		l := newTestLedger()
		require.NoError(t, l.Programs.PutProgram(ctx, testProgram()))
		require.NoError(t, l.Receipts.CreateReceipt(ctx, Receipt{
			Source: "inv-1", Month: "2026-01", GrossCents: 100000,
			Currency: "usd",
		}))

		// January: revenue but no awards → all carried forward
		jan, err := l.CloseRound(ctx, "2026-01",
			time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), "admin")
		require.NoError(t, err)
		assert.Equal(t, int64(20000), jan.UnallocatedCents)
		assert.Empty(t, jan.Entitlements)

		// February: an award activates; carryforward joins the pool
		award := approvedAward("2026-02",
			Split{Account: "alice", Credits: 10})
		_, err = l.Awards.CreateAward(ctx, award)
		require.NoError(t, err)

		feb, err := l.CloseRound(ctx, "2026-02",
			time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), "admin")
		require.NoError(t, err)
		assert.Equal(t, int64(20000), feb.CarryforwardCents)
		assert.Equal(t, int64(20000), feb.PoolCents)
		require.Len(t, feb.Entitlements, 1)
		assert.Equal(t, int64(20000), feb.Entitlements[0].AmountCents)
		assert.Zero(t, feb.UnallocatedCents)
	})

	t.Run("rejects out-of-order closing", func(t *testing.T) {
		l := setup(t)
		_, err := l.CloseRound(ctx, "2026-03", closedAt, "admin")
		require.NoError(t, err)

		// skipping April: May cannot close
		_, err = l.CloseRound(ctx, "2026-05",
			time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), "admin")
		assert.ErrorContains(t, err, "2026-04 must be closed")
	})

	t.Run("rejects closing a month that has not ended", func(t *testing.T) {
		l := setup(t)
		_, err := l.CloseRound(ctx, "2026-03",
			time.Date(2026, 3, 31, 23, 0, 0, 0, time.UTC), "admin")
		assert.ErrorContains(t, err, "has not ended")
	})

	t.Run("estimate for open month after closed rounds", func(t *testing.T) {
		l := setup(t)
		_, err := l.CloseRound(ctx, "2026-03", closedAt, "admin")
		require.NoError(t, err)

		require.NoError(t, l.Receipts.CreateReceipt(ctx, Receipt{
			Source: "inv-2", Month: "2026-04", GrossCents: 50000,
			Currency: "usd",
		}))
		round, obls, err := l.Estimate(ctx, "2026-04")
		require.NoError(t, err)
		assert.Equal(t, int64(10000), round.PoolCents)
		require.Len(t, obls, 2)
		assert.Equal(t, ObligationEstimated, obls[0].State)
	})
}
