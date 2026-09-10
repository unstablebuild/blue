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
	"time"
)

// ParticipantStatus is the eligibility status of a Participant.
type ParticipantStatus string

const (
	// ParticipantEligible participants receive entitlements in rounds.
	ParticipantEligible ParticipantStatus = "eligible"
	// ParticipantSuspended participants are temporarily excluded from
	// payouts, e.g. pending compliance review.
	ParticipantSuspended ParticipantStatus = "suspended"
	// ParticipantWithdrawn participants have left the program.
	ParticipantWithdrawn ParticipantStatus = "withdrawn"
)

// Participant is an enrolled contributor. Public recognition (the GitHub
// handle) is kept separate from private payee identity: this type carries
// only an opaque payout provider reference, never banking details.
type Participant struct {
	// Account is an opaque reference to the participant's user account
	// in the embedding system. It is the Participant's identity.
	Account string

	// GitHubHandle is the participant's public GitHub username.
	GitHubHandle string

	// AgreementVersion is the contributor agreement version accepted at
	// enrollment.
	AgreementVersion string

	// AgreementAcceptedAt is when the agreement was accepted.
	AgreementAcceptedAt time.Time

	// AgreementEvidence is an opaque reference to evidence of the
	// acceptance, e.g. a request audit log entry.
	AgreementEvidence string

	// Status is the participant's eligibility status.
	Status ParticipantStatus

	// PayoutProviderRef is an opaque reference to the participant's
	// account at the payout provider. Empty until payment onboarding
	// completes; enrollment never requires it.
	PayoutProviderRef string

	// PayoutReady reports whether the payout provider accepted the
	// participant's onboarding and transfers can be initiated.
	PayoutReady bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate returns an error if the Participant is missing required fields.
func (p Participant) Validate() error {
	if p.Account == "" {
		return errors.New("invalid participant: missing account")
	}
	if p.GitHubHandle == "" {
		return errors.New("invalid participant: missing github handle")
	}
	if p.AgreementVersion == "" {
		return errors.New("invalid participant: missing agreement version")
	}
	switch p.Status {
	case ParticipantEligible, ParticipantSuspended, ParticipantWithdrawn:
	default:
		return errors.New("invalid participant: unknown status")
	}
	return nil
}
