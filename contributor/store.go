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

	"github.com/unstablebuild/blue/iterator"
)

// ProgramStore persists Program policy versions.
type ProgramStore interface {
	// PutProgram stores the program, keyed by Program.Version.
	PutProgram(context.Context, Program) error

	// GetProgram returns the program with the given version.
	GetProgram(ctx context.Context, version string) (Program, error)

	// ProgramFor returns the policy in effect for month m: the program
	// with the latest EffectiveFrom that is not after m. It returns
	// document.ErrNotFound if no program is effective.
	ProgramFor(ctx context.Context, m Month) (Program, error)

	// ListPrograms lists all program versions.
	ListPrograms(context.Context) (iterator.Iterator[Program], error)
}

// ParticipantStore persists Participants, keyed by account reference.
type ParticipantStore interface {
	// CreateParticipant stores a new participant. It returns
	// document.ErrAlreadyExists if the account is already enrolled.
	CreateParticipant(context.Context, Participant) error

	// GetParticipant returns the participant with the given account.
	GetParticipant(ctx context.Context, account string) (Participant, error)

	// SetParticipant overwrites the participant record.
	SetParticipant(context.Context, Participant) error

	// ListParticipants lists all participants.
	ListParticipants(context.Context) (iterator.Iterator[Participant], error)
}

// AwardStore persists Awards.
type AwardStore interface {
	// CreateAward stores a new award and returns its assigned ID.
	CreateAward(context.Context, Award) (string, error)

	// GetAward returns the award with the given ID.
	GetAward(ctx context.Context, id string) (Award, error)

	// SetAward overwrites the award identified by Award.ID.
	SetAward(context.Context, Award) error

	// ListAwards lists awards with the given status, or all awards if
	// status is empty.
	ListAwards(ctx context.Context, status AwardStatus) (
		iterator.Iterator[Award], error)
}

// ReceiptStore persists Receipts, keyed by source reference so that
// imports are idempotent.
type ReceiptStore interface {
	// CreateReceipt stores a new receipt. It returns
	// document.ErrAlreadyExists if a receipt with the same source was
	// already imported.
	CreateReceipt(context.Context, Receipt) error

	// GetReceipt returns the receipt with the given source reference.
	GetReceipt(ctx context.Context, source string) (Receipt, error)

	// ListMonthReceipts lists the receipts attributed to month m.
	ListMonthReceipts(ctx context.Context, m Month) (
		iterator.Iterator[Receipt], error)
}

// RoundStore persists closed Rounds, keyed by month. Rounds are
// immutable: the interface deliberately offers no update operation.
type RoundStore interface {
	// CreateRound stores a closed round. It returns
	// document.ErrAlreadyExists if the month is already closed.
	CreateRound(context.Context, Round) error

	// GetRound returns the closed round for month m.
	GetRound(ctx context.Context, m Month) (Round, error)

	// ListRounds lists all closed rounds.
	ListRounds(context.Context) (iterator.Iterator[Round], error)
}

// ObligationStore persists Obligations, keyed by ObligationID.
type ObligationStore interface {
	// CreateObligation stores a new obligation. It returns
	// document.ErrAlreadyExists if it was already created.
	CreateObligation(context.Context, Obligation) error

	// GetObligation returns the obligation for month m and account.
	GetObligation(ctx context.Context, m Month, account string) (
		Obligation, error)

	// SetObligation overwrites the obligation record.
	SetObligation(context.Context, Obligation) error

	// ListAccountObligations lists all obligations of an account.
	ListAccountObligations(ctx context.Context, account string) (
		iterator.Iterator[Obligation], error)

	// ListStateObligations lists all obligations in the given state.
	ListStateObligations(ctx context.Context, state ObligationState) (
		iterator.Iterator[Obligation], error)
}
