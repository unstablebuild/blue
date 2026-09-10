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
	"time"
)

// ObligationState is the lifecycle state of an Obligation. States label
// where the amount stands; no state implies funds are reserved.
type ObligationState string

const (
	// ObligationEstimated is a running estimate for a round that has not
	// closed yet. Estimates are never persisted as obligations.
	ObligationEstimated ObligationState = "estimated"
	// ObligationEarned means the round closed and the entitlement is
	// final, but not yet due under the payment timetable.
	ObligationEarned ObligationState = "earned"
	// ObligationOwed means the obligation is due for payment.
	ObligationOwed ObligationState = "owed"
	// ObligationInitiated means a provider transfer was created.
	ObligationInitiated ObligationState = "initiated"
	// ObligationSettled means the transfer completed.
	ObligationSettled ObligationState = "settled"
)

// obligationTransitions enumerates the legal state transitions.
// initiated → owed covers failed transfers; settled → owed covers
// reversals. Both re-enter the payable queue explicitly rather than
// mutating history.
var obligationTransitions = map[ObligationState][]ObligationState{
	ObligationEstimated: {ObligationEarned},
	ObligationEarned:    {ObligationOwed},
	ObligationOwed:      {ObligationInitiated},
	ObligationInitiated: {ObligationSettled, ObligationOwed},
	ObligationSettled:   {ObligationOwed},
}

// ObligationEvent records one state transition for audit purposes.
type ObligationEvent struct {
	From ObligationState
	To   ObligationState
	At   time.Time
	Note string
}

// Obligation is a payable resulting from a closed round's entitlement.
// Its identity is derived from the round month and recipient account
// (see ObligationID), which also serves as the idempotency key for
// provider transfers.
type Obligation struct {
	// Account is the recipient participant's account reference.
	Account string

	// Month is the round that produced the obligation.
	Month Month

	// AmountCents is the gross amount owed in cents.
	AmountCents int64

	// Currency is the ISO 4217 currency code.
	Currency string

	// DueDate is when the obligation becomes due under the payment
	// timetable.
	DueDate time.Time

	// State is the current lifecycle state.
	State ObligationState

	// ProviderTransferRef is an opaque reference to the payout
	// provider's transfer, set once a transfer is initiated.
	ProviderTransferRef string

	// History records every state transition.
	History []ObligationEvent

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ObligationID returns the deterministic identifier of the obligation
// for month m and the given account. Determinism makes payment
// instructions idempotent.
func ObligationID(m Month, account string) string {
	return fmt.Sprintf("%s:%s", m, account)
}

// ID returns the obligation's deterministic identifier.
func (o Obligation) ID() string {
	return ObligationID(o.Month, o.Account)
}

// Transition returns a copy of the obligation moved to state, recording
// the transition in History. It returns an error if the transition is
// not legal.
func (o Obligation) Transition(
	to ObligationState, at time.Time, note string,
) (Obligation, error) {
	for _, legal := range obligationTransitions[o.State] {
		if legal != to {
			continue
		}
		history := make([]ObligationEvent, 0, len(o.History)+1)
		history = append(history, o.History...)
		history = append(history, ObligationEvent{
			From: o.State,
			To:   to,
			At:   at,
			Note: note,
		})
		o.History = history
		o.State = to
		return o, nil
	}
	return Obligation{}, fmt.Errorf(
		"obligation %s: illegal transition %s → %s", o.ID(), o.State, to)
}
