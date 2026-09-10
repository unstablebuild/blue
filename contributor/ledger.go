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
	"errors"
	"fmt"
	"time"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
)

// ErrRoundClosed is returned by Ledger.CloseRound when the month has
// already been closed.
var ErrRoundClosed = errors.New("round already closed")

// Ledger orchestrates the contributor program stores: it assembles the
// frozen inputs of a round from the stores, delegates the calculation to
// the pure engine and persists the results. It contains no allocation
// logic of its own.
type Ledger struct {
	Programs     ProgramStore
	Participants ParticipantStore
	Awards       AwardStore
	Receipts     ReceiptStore
	Rounds       RoundStore
	Obligations  ObligationStore
}

// assemble gathers the frozen inputs for month m and computes the round.
func (l Ledger) assemble(ctx context.Context, m Month) (
	Round, []Obligation, error,
) {
	if err := m.Validate(); err != nil {
		return Round{}, nil, err
	}
	program, err := l.Programs.ProgramFor(ctx, m)
	if err != nil {
		return Round{}, nil, fmt.Errorf("program for %s: %w", m, err)
	}

	receiptIt, err := l.Receipts.ListMonthReceipts(ctx, m)
	if err != nil {
		return Round{}, nil, err
	}
	receipts, err := iterator.ToSlice(ctx, receiptIt)
	if err != nil {
		return Round{}, nil, err
	}

	awardIt, err := l.Awards.ListAwards(ctx, AwardApproved)
	if err != nil {
		return Round{}, nil, err
	}
	awards, err := iterator.ToSlice(ctx, awardIt)
	if err != nil {
		return Round{}, nil, err
	}

	carryforward, err := l.carryforward(ctx, m)
	if err != nil {
		return Round{}, nil, err
	}

	return ComputeRound(program, m, receipts, awards, carryforward)
}

// carryforward returns the unallocated balance of the round preceding m.
// Rounds must be closed sequentially: if any round before m exists, the
// immediately preceding month must be closed.
func (l Ledger) carryforward(ctx context.Context, m Month) (int64, error) {
	prev, err := l.Rounds.GetRound(ctx, m.Prev())
	if err == nil {
		return prev.UnallocatedCents, nil
	}
	if err != document.ErrNotFound {
		return 0, err
	}
	// No previous round: make sure there is no earlier closed round,
	// which would mean a gap in the sequence.
	it, err := l.Rounds.ListRounds(ctx)
	if err != nil {
		return 0, err
	}
	rounds, err := iterator.ToSlice(ctx, it)
	if err != nil {
		return 0, err
	}
	for _, r := range rounds {
		if r.Month.Before(m) {
			return 0, fmt.Errorf(
				"round %s must be closed before %s", m.Prev(), m)
		}
	}
	return 0, nil
}

// Estimate computes the round for month m from current data without
// persisting anything. The returned obligations are labeled
// ObligationEstimated: nothing is final until the round closes.
func (l Ledger) Estimate(ctx context.Context, m Month) (
	Round, []Obligation, error,
) {
	round, obligations, err := l.assemble(ctx, m)
	if err != nil {
		return Round{}, nil, err
	}
	for i := range obligations {
		obligations[i].State = ObligationEstimated
	}
	return round, obligations, nil
}

// CloseRound freezes month m: it computes the round from the effective
// program, the month's receipts, the approved awards and the previous
// round's carryforward, then persists the round and its obligations in
// ObligationEarned state. It returns ErrRoundClosed if m was already
// closed. Closing out of order is rejected; a closed round is never
// modified.
func (l Ledger) CloseRound(
	ctx context.Context, m Month, closedAt time.Time, closedBy string,
) (Round, error) {
	if _, err := l.Rounds.GetRound(ctx, m); err == nil {
		return Round{}, fmt.Errorf("%w: %s", ErrRoundClosed, m)
	} else if err != document.ErrNotFound {
		return Round{}, err
	}
	// Refuse to close a month that has not ended yet: receipts could
	// still arrive.
	if closedAt.UTC().Before(m.Next().Time()) {
		return Round{}, fmt.Errorf("month %s has not ended yet", m)
	}

	round, obligations, err := l.assemble(ctx, m)
	if err != nil {
		return Round{}, err
	}
	round.ClosedAt = closedAt
	round.ClosedBy = closedBy

	for _, o := range obligations {
		o.History = []ObligationEvent{{
			From: ObligationEstimated,
			To:   ObligationEarned,
			At:   closedAt,
			Note: fmt.Sprintf("round %s closed", m),
		}}
		err := l.Obligations.CreateObligation(ctx, o)
		// Obligations are deterministic from the frozen inputs, so a
		// leftover from a previously failed close attempt is identical
		// to the one just computed and can be kept.
		if err != nil && err != document.ErrAlreadyExists {
			return Round{}, fmt.Errorf(
				"round %s: obligation %s: %v", m, o.ID(), err)
		}
	}
	// Creating the round is the commit point: obligations are persisted
	// first so a failure here leaves the close safely retryable.
	if err := l.Rounds.CreateRound(ctx, round); err != nil {
		return Round{}, err
	}
	return round, nil
}
