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
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testProgram() Program {
	return Program{
		Version:              "2026-01",
		Currency:             "usd",
		PoolBps:              2000,
		CoveredReceipts:      "all subscription revenue",
		CreditDurationRounds: 24,
		PaymentTimetable:     "30 days after quarter end",
		PayoutDueDays:        30,
		CalculationVersion:   CalculationVersion,
		EffectiveFrom:        "2026-01",
	}
}

func approvedAward(
	activation Month, splits ...Split,
) Award {
	return Award{
		Splits:        splits,
		Activation:    activation,
		Expiration:    activation.Add(24),
		Evidence:      []string{"https://github.com/unstablebuild/blue/pull/1"},
		PolicyVersion: "2026-01",
		Status:        AwardApproved,
		ProposedBy:    "admin",
	}
}

func TestActiveCredits(t *testing.T) {
	t.Run("activation and expiry boundaries", func(t *testing.T) {
		award := approvedAward("2026-02", Split{Account: "a", Credits: 10})

		for _, tc := range []struct {
			m      Month
			active bool
		}{
			{"2026-01", false}, // before activation
			{"2026-02", true},  // activation month inclusive
			{"2027-02", true},  // still active
			{"2028-01", true},  // last active month (24 rounds)
			{"2028-02", false}, // expiration month exclusive
		} {
			credits, err := ActiveCredits([]Award{award}, tc.m)
			require.NoError(t, err)
			if tc.active {
				assert.Equal(t, int64(10), credits["a"], "month %s", tc.m)
			} else {
				assert.Empty(t, credits, "month %s", tc.m)
			}
		}
	})

	t.Run("sums across awards", func(t *testing.T) {
		awards := []Award{
			approvedAward("2026-01", Split{Account: "a", Credits: 10}),
			approvedAward("2026-02",
				Split{Account: "a", Credits: 5},
				Split{Account: "b", Credits: 7}),
		}
		credits, err := ActiveCredits(awards, "2026-03")
		require.NoError(t, err)
		assert.Equal(t, map[string]int64{"a": 15, "b": 7}, credits)
	})

	t.Run("ignores proposed and rejected", func(t *testing.T) {
		proposed := approvedAward("2026-01", Split{Account: "a", Credits: 10})
		proposed.Status = AwardProposed
		rejected := approvedAward("2026-01", Split{Account: "b", Credits: 10})
		rejected.Status = AwardRejected
		credits, err := ActiveCredits([]Award{proposed, rejected}, "2026-01")
		require.NoError(t, err)
		assert.Empty(t, credits)
	})

	t.Run("rejects duplicate recipient in one award", func(t *testing.T) {
		bad := approvedAward("2026-01",
			Split{Account: "a", Credits: 10},
			Split{Account: "a", Credits: 5})
		_, err := ActiveCredits([]Award{bad}, "2026-01")
		assert.ErrorContains(t, err, "duplicate recipient")
	})
}

func TestPoolShareCents(t *testing.T) {
	assert.Equal(t, int64(2000), PoolShareCents(2000, 10000))
	// truncation toward zero: sub-cent remainder stays out of the pool
	assert.Equal(t, int64(1), PoolShareCents(2000, 9))
	assert.Equal(t, int64(0), PoolShareCents(2000, 4))
	assert.Equal(t, int64(-2000), PoolShareCents(2000, -10000))
	// no overflow on large receipts
	assert.Equal(t, int64(1844674407370955161)/5,
		PoolShareCents(2000, 1844674407370955161))
}

func TestAllocate(t *testing.T) {
	t.Run("exact proportional split", func(t *testing.T) {
		ents, err := Allocate(100, map[string]int64{"a": 3, "b": 1})
		require.NoError(t, err)
		assert.Equal(t, []Entitlement{
			{Account: "a", Credits: 3, AmountCents: 75},
			{Account: "b", Credits: 1, AmountCents: 25},
		}, ents)
	})

	t.Run("largest remainder preserves total", func(t *testing.T) {
		// 100 / 3: floor gives 33 each, 1 cent left for the largest
		// remainder; all remainders equal so first account by name wins.
		ents, err := Allocate(100, map[string]int64{"a": 1, "b": 1, "c": 1})
		require.NoError(t, err)
		assert.Equal(t, []Entitlement{
			{Account: "a", Credits: 1, AmountCents: 34},
			{Account: "b", Credits: 1, AmountCents: 33},
			{Account: "c", Credits: 1, AmountCents: 33},
		}, ents)
	})

	t.Run("largest remainder ranks by remainder", func(t *testing.T) {
		// pool 10, credits a=1 b=2 c=4 (total 7):
		// a: 10/7  = 1 rem 3
		// b: 20/7  = 2 rem 6
		// c: 40/7  = 5 rem 5
		// leftover 2 goes to b (rem 6) and c (rem 5).
		ents, err := Allocate(10, map[string]int64{"a": 1, "b": 2, "c": 4})
		require.NoError(t, err)
		assert.Equal(t, []Entitlement{
			{Account: "a", Credits: 1, AmountCents: 1},
			{Account: "b", Credits: 2, AmountCents: 3},
			{Account: "c", Credits: 4, AmountCents: 6},
		}, ents)
	})

	t.Run("zero pool records credits", func(t *testing.T) {
		ents, err := Allocate(0, map[string]int64{"a": 5, "b": 3})
		require.NoError(t, err)
		assert.Equal(t, []Entitlement{
			{Account: "a", Credits: 5, AmountCents: 0},
			{Account: "b", Credits: 3, AmountCents: 0},
		}, ents)
	})

	t.Run("zero denominator yields no entitlements", func(t *testing.T) {
		ents, err := Allocate(100, nil)
		require.NoError(t, err)
		assert.Nil(t, ents)

		ents, err = Allocate(100, map[string]int64{"a": 0})
		require.NoError(t, err)
		assert.Nil(t, ents)
	})

	t.Run("rejects negative inputs", func(t *testing.T) {
		_, err := Allocate(-1, map[string]int64{"a": 1})
		assert.Error(t, err)
		_, err = Allocate(1, map[string]int64{"a": -1})
		assert.Error(t, err)
	})

	t.Run("no int64 overflow", func(t *testing.T) {
		// pool × credits far exceeds int64
		const pool = int64(1) << 62
		ents, err := Allocate(pool, map[string]int64{
			"a": 1 << 40, "b": 1 << 40, "c": 1,
		})
		require.NoError(t, err)
		var total int64
		for _, e := range ents {
			total += e.AmountCents
		}
		assert.Equal(t, pool, total)
	})

	t.Run("deterministic and total-preserving", func(t *testing.T) {
		rng := rand.New(rand.NewSource(42))
		for i := 0; i < 200; i++ {
			pool := rng.Int63n(1 << 40)
			credits := make(map[string]int64)
			for j := 0; j < 1+rng.Intn(20); j++ {
				credits[fmt.Sprintf("acct-%d", j)] = rng.Int63n(1000)
			}
			first, err := Allocate(pool, credits)
			require.NoError(t, err)
			again, err := Allocate(pool, credits)
			require.NoError(t, err)
			assert.Equal(t, first, again, "iteration %d not deterministic", i)

			var totalCredits, allocated int64
			for _, c := range credits {
				totalCredits += c
			}
			for _, e := range first {
				allocated += e.AmountCents
			}
			if totalCredits == 0 {
				assert.Nil(t, first)
				continue
			}
			require.Equal(t, pool, allocated,
				"iteration %d: allocated %d != pool %d", i, allocated, pool)
			// no participant deviates more than one cent from the
			// unrounded proportional share
			for _, e := range first {
				exact := float64(pool) * float64(e.Credits) /
					float64(totalCredits)
				assert.InDelta(t, exact, float64(e.AmountCents), 1.0)
			}
		}
	})
}

func TestDueDate(t *testing.T) {
	p := testProgram() // due 30 days after quarter end
	for _, tc := range []struct {
		m    Month
		want time.Time
	}{
		{"2026-01", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{"2026-03", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{"2026-04", time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
		{"2026-10", time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)},
		{"2026-12", time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)},
	} {
		assert.Equal(t, tc.want, DueDate(p, tc.m), "month %s", tc.m)
	}
}

func TestComputeRound(t *testing.T) {
	program := testProgram()
	m := Month("2026-03")
	receipts := []Receipt{
		{
			Source: "inv-1", Month: m, GrossCents: 100000,
			Currency: "usd",
			Adjustments: []Adjustment{
				{Kind: AdjustmentTax, AmountCents: 10000},
			},
		},
		{Source: "inv-2", Month: m, GrossCents: 50000, Currency: "usd"},
	}
	awards := []Award{
		approvedAward("2026-01", Split{Account: "a", Credits: 60}),
		approvedAward("2026-02", Split{Account: "b", Credits: 40}),
	}

	t.Run("happy path", func(t *testing.T) {
		round, obls, err := ComputeRound(program, m, receipts, awards, 0)
		require.NoError(t, err)

		// net = 90000 + 50000; pool = 20% = 28000
		assert.Equal(t, int64(140000), round.NetReceiptsCents)
		assert.Equal(t, int64(28000), round.PoolCents)
		assert.Equal(t, int64(100), round.TotalCredits)
		assert.Equal(t, int64(0), round.UnallocatedCents)
		assert.Equal(t, CalculationVersion, round.CalculationVersion)
		assert.Equal(t, program.Version, round.PolicyVersion)
		assert.Equal(t, []Entitlement{
			{Account: "a", Credits: 60, AmountCents: 16800},
			{Account: "b", Credits: 40, AmountCents: 11200},
		}, round.Entitlements)

		require.Len(t, obls, 2)
		assert.Equal(t, ObligationEarned, obls[0].State)
		assert.Equal(t, int64(16800), obls[0].AmountCents)
		assert.Equal(t, DueDate(program, m), obls[0].DueDate)
		assert.Equal(t, "2026-03:a", obls[0].ID())

		// the closed round verifies
		require.NoError(t, VerifyRound(Statement{
			Program: program, Round: round,
		}))
	})

	t.Run("zero receipts", func(t *testing.T) {
		round, obls, err := ComputeRound(program, m, nil, awards, 0)
		require.NoError(t, err)
		assert.Zero(t, round.PoolCents)
		assert.Equal(t, int64(100), round.TotalCredits)
		// zero-amount entitlements still freeze the credits
		require.Len(t, round.Entitlements, 2)
		assert.Zero(t, round.Entitlements[0].AmountCents)
		assert.Empty(t, obls)
		require.NoError(t, VerifyRound(Statement{
			Program: program, Round: round,
		}))
	})

	t.Run("zero denominator carries pool forward", func(t *testing.T) {
		round, obls, err := ComputeRound(program, m, receipts, nil, 500)
		require.NoError(t, err)
		assert.Equal(t, int64(28500), round.PoolCents)
		assert.Zero(t, round.TotalCredits)
		assert.Empty(t, round.Entitlements)
		assert.Equal(t, int64(28500), round.UnallocatedCents)
		assert.Empty(t, obls)
		require.NoError(t, VerifyRound(Statement{
			Program: program, Round: round,
		}))
	})

	t.Run("carryforward joins pool", func(t *testing.T) {
		round, _, err := ComputeRound(program, m, receipts, awards, 1000)
		require.NoError(t, err)
		assert.Equal(t, int64(29000), round.PoolCents)
		assert.Equal(t, int64(1000), round.CarryforwardCents)
		assert.Equal(t, int64(0), round.UnallocatedCents)
	})

	t.Run("negative pool carries forward", func(t *testing.T) {
		refund := []Receipt{{
			Source: "ref-1", Month: m, GrossCents: 1000,
			Currency: "usd",
			Adjustments: []Adjustment{
				{Kind: AdjustmentRefund, AmountCents: 51000},
			},
		}}
		round, obls, err := ComputeRound(program, m, refund, awards, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(-10000), round.PoolCents)
		assert.Equal(t, int64(-10000), round.UnallocatedCents)
		require.Len(t, round.Entitlements, 2)
		assert.Zero(t, round.Entitlements[0].AmountCents)
		assert.Empty(t, obls)
		require.NoError(t, VerifyRound(Statement{
			Program: program, Round: round,
		}))
	})

	t.Run("rejects foreign receipts", func(t *testing.T) {
		other := []Receipt{{
			Source: "inv-9", Month: "2026-02", GrossCents: 100,
			Currency: "usd",
		}}
		_, _, err := ComputeRound(program, m, other, awards, 0)
		assert.ErrorContains(t, err, "does not belong to round")

		eur := []Receipt{{
			Source: "inv-8", Month: m, GrossCents: 100, Currency: "eur",
		}}
		_, _, err = ComputeRound(program, m, eur, awards, 0)
		assert.ErrorContains(t, err, "currency")
	})

	t.Run("rejects engine version mismatch", func(t *testing.T) {
		stale := program
		stale.CalculationVersion = CalculationVersion + 1
		_, _, err := ComputeRound(stale, m, receipts, awards, 0)
		assert.ErrorContains(t, err, "calculation version")
	})

	t.Run("rejects month before policy", func(t *testing.T) {
		_, _, err := ComputeRound(program, "2025-12", nil, nil, 0)
		assert.ErrorContains(t, err, "not effective")
	})
}

func TestVerifyRoundDetectsTampering(t *testing.T) {
	program := testProgram()
	m := Month("2026-03")
	receipts := []Receipt{{
		Source: "inv-1", Month: m, GrossCents: 100001, Currency: "usd",
	}}
	awards := []Award{
		approvedAward("2026-01", Split{Account: "a", Credits: 7}),
		approvedAward("2026-01", Split{Account: "b", Credits: 3}),
	}
	round, _, err := ComputeRound(program, m, receipts, awards, 0)
	require.NoError(t, err)
	require.NoError(t, VerifyRound(Statement{Program: program, Round: round}))

	t.Run("tampered amount", func(t *testing.T) {
		bad := round
		bad.Entitlements = append([]Entitlement(nil), round.Entitlements...)
		bad.Entitlements[0].AmountCents++
		err := VerifyRound(Statement{Program: program, Round: bad})
		assert.ErrorContains(t, err, "recomputed")
	})

	t.Run("tampered pool", func(t *testing.T) {
		bad := round
		bad.PoolCents++
		err := VerifyRound(Statement{Program: program, Round: bad})
		assert.ErrorContains(t, err, "pool")
	})

	t.Run("tampered denominator", func(t *testing.T) {
		bad := round
		bad.TotalCredits++
		err := VerifyRound(Statement{Program: program, Round: bad})
		assert.ErrorContains(t, err, "denominator")
	})

	t.Run("tampered credits", func(t *testing.T) {
		bad := round
		bad.Entitlements = append([]Entitlement(nil), round.Entitlements...)
		bad.Entitlements[0].Credits += 2
		bad.TotalCredits += 2
		err := VerifyRound(Statement{Program: program, Round: bad})
		assert.Error(t, err)
	})
}
