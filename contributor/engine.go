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
	"math/big"
	"sort"
	"time"
)

// CalculationVersion is the version of the allocation engine implemented
// by this package. Programs pin the calculation version they were
// published against; ComputeRound refuses to run on a mismatch.
const CalculationVersion = 1

// The functions in this file are pure and deterministic: given the same
// inputs they always produce the same outputs, so any closed round can
// be recomputed and verified independently.

// ActiveCredits resolves the active credits per account for month m from
// approved awards. An award is active in m when
// Activation <= m < Expiration. Awards that are not approved are
// ignored; approved awards that fail validation are an error.
func ActiveCredits(awards []Award, m Month) (map[string]int64, error) {
	credits := make(map[string]int64)
	for _, a := range awards {
		if a.Status != AwardApproved {
			continue
		}
		if err := a.Validate(); err != nil {
			return nil, fmt.Errorf("award %s: %v", a.ID, err)
		}
		if !a.ActiveIn(m) {
			continue
		}
		for _, s := range a.Splits {
			credits[s.Account] += s.Credits
		}
	}
	return credits, nil
}

// PoolShareCents returns the pool contribution of the given net receipts
// under a pool share expressed in basis points. The result is truncated
// toward zero; sub-cent remainders stay with the operator, never the
// pool.
func PoolShareCents(poolBps int, netReceiptsCents int64) int64 {
	share := new(big.Int).Mul(
		big.NewInt(netReceiptsCents), big.NewInt(int64(poolBps)))
	return share.Quo(share, big.NewInt(10000)).Int64()
}

// Allocate splits poolCents among accounts proportionally to their
// credits: amount_i = poolCents × credits_i / Σcredits, rounded with the
// largest-remainder method so that the allocated total always equals
// poolCents exactly. Entitlements are returned ordered by account.
//
// A zero credit total yields no entitlements: the caller carries the
// pool forward. A zero pool yields zero-amount entitlements that still
// record each account's credits.
func Allocate(poolCents int64, credits map[string]int64) ([]Entitlement, error) {
	if poolCents < 0 {
		return nil, fmt.Errorf("allocate: negative pool %d", poolCents)
	}
	accounts := make([]string, 0, len(credits))
	var total int64
	for account, c := range credits {
		if c < 0 {
			return nil, fmt.Errorf(
				"allocate: negative credits for %q", account)
		}
		if c == 0 {
			continue
		}
		accounts = append(accounts, account)
		total += c
	}
	if total == 0 {
		return nil, nil
	}
	sort.Strings(accounts)

	ents := make([]Entitlement, len(accounts))
	remainders := make([]*big.Int, len(accounts))
	pool := big.NewInt(poolCents)
	denominator := big.NewInt(total)
	var allocated int64
	for i, account := range accounts {
		num := new(big.Int).Mul(pool, big.NewInt(credits[account]))
		quo, rem := num.QuoRem(num, denominator, new(big.Int))
		ents[i] = Entitlement{
			Account:     account,
			Credits:     credits[account],
			AmountCents: quo.Int64(),
		}
		remainders[i] = rem
		allocated += ents[i].AmountCents
	}

	// Distribute the remaining cents to the largest remainders,
	// breaking ties by account order for determinism.
	leftover := poolCents - allocated
	order := make([]int, len(ents))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return remainders[order[a]].Cmp(remainders[order[b]]) > 0
	})
	for i := int64(0); i < leftover; i++ {
		ents[order[i]].AmountCents++
	}
	return ents, nil
}

// DueDate returns when obligations of round m become due under policy p:
// PayoutDueDays days after the end of the calendar quarter containing m.
func DueDate(p Program, m Month) time.Time {
	t := m.Time()
	quarter := (int(t.Month()) - 1) / 3
	// First instant after the quarter; time.Date normalizes month
	// overflow into the next year.
	end := time.Date(t.Year(), time.Month(quarter*3+4), 1, 0, 0, 0, 0,
		time.UTC)
	return end.AddDate(0, 0, p.PayoutDueDays)
}

// ComputeRound computes the allocation round for month m from frozen
// inputs: the effective program, the month's covered receipts, the
// approved awards and the balance carried forward from the previous
// round. It returns the Round and the obligations it produces, in
// ObligationEarned state. It persists nothing and reads no clocks:
// closing is the caller's responsibility.
//
// When the pool is not positive or no credits are active, the round
// carries its pool forward through Round.UnallocatedCents and produces
// no obligations.
func ComputeRound(
	p Program, m Month, receipts []Receipt, awards []Award,
	carryforwardCents int64,
) (Round, []Obligation, error) {
	if err := p.Validate(); err != nil {
		return Round{}, nil, err
	}
	if p.CalculationVersion != CalculationVersion {
		return Round{}, nil, fmt.Errorf(
			"program %s: calculation version %d does not match engine %d",
			p.Version, p.CalculationVersion, CalculationVersion)
	}
	if err := m.Validate(); err != nil {
		return Round{}, nil, err
	}
	if m.Before(p.EffectiveFrom) {
		return Round{}, nil, fmt.Errorf(
			"program %s is not effective in %s", p.Version, m)
	}

	var net int64
	for _, r := range receipts {
		if err := r.Validate(); err != nil {
			return Round{}, nil, err
		}
		if r.Month != m {
			return Round{}, nil, fmt.Errorf(
				"receipt %q: month %s does not belong to round %s",
				r.Source, r.Month, m)
		}
		if r.Currency != p.Currency {
			return Round{}, nil, fmt.Errorf(
				"receipt %q: currency %q does not match program currency %q",
				r.Source, r.Currency, p.Currency)
		}
		net += r.NetCents()
	}

	credits, err := ActiveCredits(awards, m)
	if err != nil {
		return Round{}, nil, err
	}
	var totalCredits int64
	for _, c := range credits {
		totalCredits += c
	}

	pool := carryforwardCents + PoolShareCents(p.PoolBps, net)
	allocatable := pool
	if allocatable < 0 {
		allocatable = 0
	}
	ents, err := Allocate(allocatable, credits)
	if err != nil {
		return Round{}, nil, err
	}

	round := Round{
		Month:              m,
		PolicyVersion:      p.Version,
		CalculationVersion: CalculationVersion,
		Currency:           p.Currency,
		NetReceiptsCents:   net,
		CarryforwardCents:  carryforwardCents,
		PoolCents:          pool,
		TotalCredits:       totalCredits,
		Entitlements:       ents,
	}
	round.UnallocatedCents = pool - round.AllocatedCents()

	due := DueDate(p, m)
	var obligations []Obligation
	for _, e := range ents {
		if e.AmountCents <= 0 {
			continue
		}
		obligations = append(obligations, Obligation{
			Account:     e.Account,
			Month:       m,
			AmountCents: e.AmountCents,
			Currency:    p.Currency,
			DueDate:     due,
			State:       ObligationEarned,
		})
	}
	return round, obligations, nil
}
