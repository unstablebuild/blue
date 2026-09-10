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

import "time"

// Entitlement is one participant's share of a round's pool.
type Entitlement struct {
	// Account is the participant's account reference.
	Account string
	// Credits is the participant's active credits in the round —
	// a frozen input of the calculation.
	Credits int64
	// AmountCents is the resulting entitlement in cents.
	AmountCents int64
}

// Round is a closed monthly allocation. All inputs are frozen at closing
// time so the calculation can be reproduced and verified independently
// (see VerifyRound). A closed Round is immutable: corrections are new
// events (e.g. adjustment receipts in a later round), never edits.
type Round struct {
	// Month identifies the round.
	Month Month

	// PolicyVersion is the Program.Version applied.
	PolicyVersion string

	// CalculationVersion is the engine version used to compute the
	// round.
	CalculationVersion int

	// Currency is the program currency the round is denominated in.
	Currency string

	// NetReceiptsCents is the sum of covered net receipts of the month.
	NetReceiptsCents int64

	// CarryforwardCents is the unallocated balance carried in from the
	// previous round.
	CarryforwardCents int64

	// PoolCents is the allocatable pool:
	// CarryforwardCents + NetReceiptsCents × PoolBps / 10000.
	PoolCents int64

	// TotalCredits is the allocation denominator: the sum of all active
	// credits in the round.
	TotalCredits int64

	// Entitlements are the per-participant results, ordered by account.
	Entitlements []Entitlement

	// UnallocatedCents is the part of the pool not allocated in this
	// round; it is carried forward into the next round. It is the whole
	// pool when TotalCredits is zero and zero otherwise.
	UnallocatedCents int64

	// ClosedAt is when the round was closed.
	ClosedAt time.Time
	// ClosedBy identifies who closed the round.
	ClosedBy string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// AllocatedCents returns the sum of the round's entitlement amounts.
func (r Round) AllocatedCents() int64 {
	var total int64
	for _, e := range r.Entitlements {
		total += e.AmountCents
	}
	return total
}
