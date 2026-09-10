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

func testParticipant(account string) Participant {
	return Participant{
		Account:             account,
		GitHubHandle:        "gh-" + account,
		AgreementVersion:    "v1",
		AgreementAcceptedAt: time.Now().UTC(),
		Status:              ParticipantEligible,
	}
}

func TestDocumentProgramStore(t *testing.T) {
	ctx := context.Background()
	s := NewDocumentProgramStore(document.NewInMemoryService())

	v1 := testProgram()
	require.NoError(t, s.PutProgram(ctx, v1))
	v2 := testProgram()
	v2.Version = "2026-06"
	v2.EffectiveFrom = "2026-06"
	v2.PoolBps = 2500
	require.NoError(t, s.PutProgram(ctx, v2))

	got, err := s.GetProgram(ctx, "2026-01")
	require.NoError(t, err)
	assert.Equal(t, 2000, got.PoolBps)

	t.Run("rejects invalid", func(t *testing.T) {
		bad := testProgram()
		bad.PoolBps = 0
		assert.Error(t, s.PutProgram(ctx, bad))
	})

	t.Run("program for month", func(t *testing.T) {
		p, err := s.ProgramFor(ctx, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, "2026-01", p.Version)

		p, err = s.ProgramFor(ctx, "2026-06")
		require.NoError(t, err)
		assert.Equal(t, "2026-06", p.Version)

		p, err = s.ProgramFor(ctx, "2027-01")
		require.NoError(t, err)
		assert.Equal(t, "2026-06", p.Version)

		_, err = s.ProgramFor(ctx, "2025-12")
		assert.Equal(t, document.ErrNotFound, err)
	})
}

func TestDocumentParticipantStore(t *testing.T) {
	ctx := context.Background()
	s := NewDocumentParticipantStore(document.NewInMemoryService())

	p := testParticipant("auth0|alice")
	require.NoError(t, s.CreateParticipant(ctx, p))

	t.Run("duplicate enrollment rejected", func(t *testing.T) {
		assert.Equal(t, document.ErrAlreadyExists,
			s.CreateParticipant(ctx, p))
	})

	t.Run("get and update", func(t *testing.T) {
		got, err := s.GetParticipant(ctx, "auth0|alice")
		require.NoError(t, err)
		assert.Equal(t, "gh-auth0|alice", got.GitHubHandle)

		got.PayoutProviderRef = "acct_123"
		got.PayoutReady = true
		require.NoError(t, s.SetParticipant(ctx, got))

		got, err = s.GetParticipant(ctx, "auth0|alice")
		require.NoError(t, err)
		assert.True(t, got.PayoutReady)
		assert.Equal(t, "acct_123", got.PayoutProviderRef)
	})

	t.Run("list", func(t *testing.T) {
		require.NoError(t, s.CreateParticipant(ctx,
			testParticipant("auth0|bob")))
		it, err := s.ListParticipants(ctx)
		require.NoError(t, err)
		all, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		assert.Len(t, all, 2)
	})

	t.Run("rejects invalid", func(t *testing.T) {
		assert.Error(t, s.CreateParticipant(ctx, Participant{}))
	})
}

func TestDocumentAwardStore(t *testing.T) {
	ctx := context.Background()
	s := NewDocumentAwardStore(document.NewInMemoryService())

	a := approvedAward("2026-01", Split{Account: "alice", Credits: 10})
	a.Status = AwardProposed
	id, err := s.CreateAward(ctx, a)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	t.Run("get carries assigned ID", func(t *testing.T) {
		got, err := s.GetAward(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, got.ID)
		assert.Equal(t, AwardProposed, got.Status)
	})

	t.Run("approve and filter by status", func(t *testing.T) {
		got, err := s.GetAward(ctx, id)
		require.NoError(t, err)
		got.Status = AwardApproved
		got.DecidedBy = "admin"
		got.DecidedAt = time.Now().UTC()
		require.NoError(t, s.SetAward(ctx, got))

		rejected := approvedAward("2026-01",
			Split{Account: "bob", Credits: 5})
		rejected.Status = AwardRejected
		_, err = s.CreateAward(ctx, rejected)
		require.NoError(t, err)

		it, err := s.ListAwards(ctx, AwardApproved)
		require.NoError(t, err)
		approved, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		require.Len(t, approved, 1)
		assert.Equal(t, id, approved[0].ID)

		it, err = s.ListAwards(ctx, "")
		require.NoError(t, err)
		all, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		assert.Len(t, all, 2)
	})

	t.Run("rejects client-provided ID", func(t *testing.T) {
		bad := approvedAward("2026-01", Split{Account: "x", Credits: 1})
		bad.ID = "custom"
		_, err := s.CreateAward(ctx, bad)
		assert.Error(t, err)
	})

	t.Run("rejects invalid", func(t *testing.T) {
		bad := approvedAward("2026-01")
		_, err := s.CreateAward(ctx, bad)
		assert.ErrorContains(t, err, "no recipients")
	})
}

func TestDocumentReceiptStore(t *testing.T) {
	ctx := context.Background()
	s := NewDocumentReceiptStore(document.NewInMemoryService())

	r := Receipt{
		Source: "inv-1", Month: "2026-03", GrossCents: 1000,
		Currency: "usd",
	}
	require.NoError(t, s.CreateReceipt(ctx, r))

	t.Run("import is idempotent", func(t *testing.T) {
		assert.Equal(t, document.ErrAlreadyExists, s.CreateReceipt(ctx, r))
	})

	t.Run("list by month", func(t *testing.T) {
		require.NoError(t, s.CreateReceipt(ctx, Receipt{
			Source: "inv-2", Month: "2026-03", GrossCents: 500,
			Currency: "usd",
		}))
		require.NoError(t, s.CreateReceipt(ctx, Receipt{
			Source: "inv-3", Month: "2026-04", GrossCents: 700,
			Currency: "usd",
		}))

		it, err := s.ListMonthReceipts(ctx, "2026-03")
		require.NoError(t, err)
		march, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		assert.Len(t, march, 2)
	})

	t.Run("get", func(t *testing.T) {
		got, err := s.GetReceipt(ctx, "inv-1")
		require.NoError(t, err)
		assert.Equal(t, int64(1000), got.GrossCents)
	})
}

func TestDocumentRoundStore(t *testing.T) {
	ctx := context.Background()
	s := NewDocumentRoundStore(document.NewInMemoryService())

	r := Round{Month: "2026-03", PoolCents: 100, Currency: "usd"}
	require.NoError(t, s.CreateRound(ctx, r))

	t.Run("rounds are immutable", func(t *testing.T) {
		assert.Equal(t, document.ErrAlreadyExists, s.CreateRound(ctx, r))
	})

	t.Run("get and list", func(t *testing.T) {
		got, err := s.GetRound(ctx, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, int64(100), got.PoolCents)

		_, err = s.GetRound(ctx, "2026-04")
		assert.Equal(t, document.ErrNotFound, err)

		it, err := s.ListRounds(ctx)
		require.NoError(t, err)
		all, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		assert.Len(t, all, 1)
	})
}

func TestDocumentObligationStore(t *testing.T) {
	ctx := context.Background()
	s := NewDocumentObligationStore(document.NewInMemoryService())

	o := Obligation{
		Account: "alice", Month: "2026-03", AmountCents: 1000,
		Currency: "usd", State: ObligationEarned,
	}
	require.NoError(t, s.CreateObligation(ctx, o))

	t.Run("idempotent creation", func(t *testing.T) {
		assert.Equal(t, document.ErrAlreadyExists,
			s.CreateObligation(ctx, o))
	})

	t.Run("state transitions persist", func(t *testing.T) {
		got, err := s.GetObligation(ctx, "2026-03", "alice")
		require.NoError(t, err)
		owed, err := got.Transition(ObligationOwed, time.Now().UTC(), "")
		require.NoError(t, err)
		require.NoError(t, s.SetObligation(ctx, owed))

		got, err = s.GetObligation(ctx, "2026-03", "alice")
		require.NoError(t, err)
		assert.Equal(t, ObligationOwed, got.State)
		assert.Len(t, got.History, 1)
	})

	t.Run("list by account and state", func(t *testing.T) {
		require.NoError(t, s.CreateObligation(ctx, Obligation{
			Account: "alice", Month: "2026-04", AmountCents: 500,
			Currency: "usd", State: ObligationEarned,
		}))
		require.NoError(t, s.CreateObligation(ctx, Obligation{
			Account: "bob", Month: "2026-04", AmountCents: 700,
			Currency: "usd", State: ObligationEarned,
		}))

		it, err := s.ListAccountObligations(ctx, "alice")
		require.NoError(t, err)
		alice, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		assert.Len(t, alice, 2)

		it, err = s.ListStateObligations(ctx, ObligationOwed)
		require.NoError(t, err)
		owed, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		require.Len(t, owed, 1)
		assert.Equal(t, "alice", owed[0].Account)
		assert.Equal(t, Month("2026-03"), owed[0].Month)
	})
}
