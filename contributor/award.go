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

// AwardStatus is the review status of an Award.
type AwardStatus string

const (
	// AwardProposed awards are pending review and carry no weight.
	AwardProposed AwardStatus = "proposed"
	// AwardApproved awards contribute active credits between their
	// activation and expiration months.
	AwardApproved AwardStatus = "approved"
	// AwardRejected awards were reviewed and declined.
	AwardRejected AwardStatus = "rejected"
)

// Split assigns a share of an Award's credits to one recipient.
type Split struct {
	// Account is the recipient participant's account reference.
	Account string
	// Credits is the credit quantity assigned to the recipient.
	// Credits are allocation weights, not currency.
	Credits int64
}

// Award is an explicit grant of credits for a contribution, always backed
// by public evidence (e.g. merged pull request URLs). Awards are never
// auto-computed from activity metrics.
type Award struct {
	// ID is the award's identifier, assigned by the store on creation.
	ID string

	// Splits assigns the award's credits to one or more recipients.
	Splits []Split

	// Activation is the first round in which the credits are active.
	Activation Month

	// Expiration is the first round in which the credits are no longer
	// active (exclusive bound), normally derived from the policy's
	// credit duration via Program.ExpirationMonth.
	Expiration Month

	// Evidence lists public URLs backing the award, e.g. merged pull
	// requests. At least one is required.
	Evidence []string

	// Description summarizes the contribution being awarded.
	Description string

	// PolicyVersion is the Program.Version under which the award was
	// proposed.
	PolicyVersion string

	// Status is the award's review status.
	Status AwardStatus

	// ProposedBy identifies who proposed the award.
	ProposedBy string
	// ProposedAt is when the award was proposed.
	ProposedAt time.Time

	// DecidedBy identifies who approved or rejected the award.
	DecidedBy string
	// DecidedAt is when the award was approved or rejected.
	DecidedAt time.Time
	// DecisionNote optionally explains the decision.
	DecisionNote string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate returns an error if the Award is not well formed. Recipients
// must be unique within an award and every split must carry positive
// credits.
func (a Award) Validate() error {
	if len(a.Splits) == 0 {
		return errors.New("invalid award: no recipients")
	}
	seen := make(map[string]bool, len(a.Splits))
	for _, s := range a.Splits {
		if s.Account == "" {
			return errors.New("invalid award: split missing account")
		}
		if s.Credits <= 0 {
			return fmt.Errorf(
				"invalid award: split for %q has non-positive credits",
				s.Account)
		}
		if seen[s.Account] {
			return fmt.Errorf(
				"invalid award: duplicate recipient %q", s.Account)
		}
		seen[s.Account] = true
	}
	if err := a.Activation.Validate(); err != nil {
		return fmt.Errorf("invalid award: activation: %v", err)
	}
	if err := a.Expiration.Validate(); err != nil {
		return fmt.Errorf("invalid award: expiration: %v", err)
	}
	if !a.Activation.Before(a.Expiration) {
		return fmt.Errorf(
			"invalid award: expiration %s not after activation %s",
			a.Expiration, a.Activation)
	}
	if len(a.Evidence) == 0 {
		return errors.New("invalid award: missing evidence")
	}
	if a.PolicyVersion == "" {
		return errors.New("invalid award: missing policy version")
	}
	switch a.Status {
	case AwardProposed, AwardApproved, AwardRejected:
	default:
		return errors.New("invalid award: unknown status")
	}
	return nil
}

// TotalCredits returns the sum of the award's split credits.
func (a Award) TotalCredits() int64 {
	var total int64
	for _, s := range a.Splits {
		total += s.Credits
	}
	return total
}

// ActiveIn reports whether the award's credits are active in month m,
// i.e. Activation <= m < Expiration.
func (a Award) ActiveIn(m Month) bool {
	return !m.Before(a.Activation) && m.Before(a.Expiration)
}
