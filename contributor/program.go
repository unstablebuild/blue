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
	"errors"
	"fmt"
	"time"
)

// Program is a versioned policy of the contributor program. A Program is
// immutable once published: policy changes are new Program versions with a
// later EffectiveFrom month.
type Program struct {
	// Version identifies this policy revision, e.g. "2026-01".
	Version string

	// Currency is the ISO 4217 currency code all receipts, pools and
	// obligations are denominated in, e.g. "usd".
	Currency string

	// PoolBps is the share of covered net receipts allocated to the
	// monthly pool, in basis points (2000 = 20%).
	PoolBps int

	// CoveredReceipts is a human-readable definition of which receipts
	// are covered by the program.
	CoveredReceipts string

	// CreditDurationRounds is the number of monthly rounds an awarded
	// credit stays active, counted from its activation month.
	CreditDurationRounds int

	// PaymentTimetable is a human-readable description of when earned
	// entitlements become owed and are paid out.
	PaymentTimetable string

	// PayoutDueDays is the number of days after the end of the calendar
	// quarter containing a round in which its obligations become due.
	PayoutDueDays int

	// CalculationVersion is the allocation engine version this policy
	// was published against. Rounds refuse to close if it does not match
	// the engine's CalculationVersion.
	CalculationVersion int

	// EffectiveFrom is the first month this policy applies to.
	EffectiveFrom Month

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate returns an error if the Program is not a usable policy.
func (p Program) Validate() error {
	if p.Version == "" {
		return errors.New("invalid program: missing version")
	}
	if p.Currency == "" {
		return errors.New("invalid program: missing currency")
	}
	if p.PoolBps <= 0 || p.PoolBps > 10000 {
		return fmt.Errorf(
			"invalid program: pool basis points %d out of range (0, 10000]",
			p.PoolBps)
	}
	if p.CreditDurationRounds <= 0 {
		return errors.New("invalid program: credit duration must be positive")
	}
	if p.PayoutDueDays <= 0 {
		return errors.New("invalid program: payout due days must be positive")
	}
	if err := p.EffectiveFrom.Validate(); err != nil {
		return fmt.Errorf("invalid program: effective from: %v", err)
	}
	return nil
}

// ExpirationMonth returns the first month in which a credit activated in
// the given month is no longer active under this policy.
func (p Program) ExpirationMonth(activation Month) Month {
	return activation.Add(p.CreditDurationRounds)
}
