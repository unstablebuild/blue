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
	"strings"
	"time"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
)

// Administrative operations fail with these sentinels so callers can map
// them to transport-specific outcomes without matching on message text.
var (
	// ErrProgramNotConfigured is returned when no program policy is
	// effective for the month an operation applies to.
	ErrProgramNotConfigured = errors.New("no program effective")

	// ErrParticipantNotFound is returned when an award names a GitHub
	// handle that belongs to no enrolled participant.
	ErrParticipantNotFound = errors.New("no enrolled participant")

	// ErrAwardNotFound is returned when a decision names an unknown
	// award.
	ErrAwardNotFound = errors.New("award not found")

	// ErrAwardAlreadyDecided is returned when a decision targets an
	// award that has already been approved or rejected. Decisions are
	// final.
	ErrAwardAlreadyDecided = errors.New("award already decided")
)

// SetProgram publishes a program policy version. The policy is pinned to
// the engine's CalculationVersion so a round computed under it refuses to
// close against a different engine.
func (l Ledger) SetProgram(ctx context.Context, p Program) error {
	p.CalculationVersion = CalculationVersion
	if err := p.Validate(); err != nil {
		return err
	}
	return l.Programs.PutProgram(ctx, p)
}

// HandleSplit assigns a share of an award's credits to a recipient
// identified by GitHub handle.
type HandleSplit struct {
	// GitHubHandle is the recipient's public GitHub username. It is
	// matched against enrolled participants case-insensitively.
	GitHubHandle string

	// Credits is the credit quantity assigned to the recipient.
	Credits int64
}

// ProposeAwardRequest describes an award to propose. Recipients are named
// by GitHub handle so operators never handle opaque account references.
type ProposeAwardRequest struct {
	// Splits assigns the award's credits to one or more recipients.
	Splits []HandleSplit

	// Activation is the first round in which the credits are active.
	Activation Month

	// Evidence lists public URLs backing the award. At least one is
	// required.
	Evidence []string

	// Description summarizes the contribution being awarded.
	Description string

	// ProposedBy identifies who proposed the award.
	ProposedBy string
}

// ProposeAward records a proposed award and returns its assigned ID. The
// award's expiration and policy version are derived from the program
// effective in the activation month, so a later policy change never
// retroactively alters the terms an award was granted under.
//
// It returns ErrProgramNotConfigured if no policy is effective in the
// activation month and ErrParticipantNotFound if a split names a handle
// that is not enrolled.
func (l Ledger) ProposeAward(
	ctx context.Context, req ProposeAwardRequest,
) (string, error) {
	if err := req.Activation.Validate(); err != nil {
		return "", err
	}
	program, err := l.Programs.ProgramFor(ctx, req.Activation)
	if err != nil {
		if errors.Is(err, document.ErrNotFound) {
			return "", fmt.Errorf("%w in activation month %s",
				ErrProgramNotConfigured, req.Activation)
		}
		return "", fmt.Errorf("program for %s: %w", req.Activation, err)
	}

	splits, err := l.resolveSplits(ctx, req.Splits)
	if err != nil {
		return "", err
	}

	return l.Awards.CreateAward(ctx, Award{
		Splits:        splits,
		Activation:    req.Activation,
		Expiration:    program.ExpirationMonth(req.Activation),
		Evidence:      req.Evidence,
		Description:   req.Description,
		PolicyVersion: program.Version,
		Status:        AwardProposed,
		ProposedBy:    req.ProposedBy,
		ProposedAt:    time.Now().UTC(),
	})
}

// resolveSplits maps GitHub handles to the account references of enrolled
// participants. Handles are compared case-insensitively because GitHub
// usernames are.
func (l Ledger) resolveSplits(
	ctx context.Context, in []HandleSplit,
) ([]Split, error) {
	it, err := l.Participants.ListParticipants(ctx)
	if err != nil {
		return nil, err
	}
	participants, err := iterator.ToSlice(ctx, it)
	if err != nil {
		return nil, err
	}
	accountByHandle := make(map[string]string, len(participants))
	for _, p := range participants {
		accountByHandle[strings.ToLower(p.GitHubHandle)] = p.Account
	}

	splits := make([]Split, 0, len(in))
	for _, s := range in {
		account, ok := accountByHandle[strings.ToLower(s.GitHubHandle)]
		if !ok {
			return nil, fmt.Errorf("%w with GitHub handle %q",
				ErrParticipantNotFound, s.GitHubHandle)
		}
		splits = append(splits, Split{Account: account, Credits: s.Credits})
	}
	return splits, nil
}

// DecideAward approves or rejects a proposed award and returns the
// decided record. Only proposed awards can be decided: it returns
// ErrAwardNotFound for an unknown ID and ErrAwardAlreadyDecided if the
// award was already approved or rejected.
func (l Ledger) DecideAward(
	ctx context.Context, id string, approve bool, decidedBy, note string,
) (Award, error) {
	award, err := l.Awards.GetAward(ctx, id)
	if err != nil {
		if errors.Is(err, document.ErrNotFound) {
			return Award{}, fmt.Errorf("%w: %s", ErrAwardNotFound, id)
		}
		return Award{}, err
	}
	if award.Status != AwardProposed {
		return Award{}, fmt.Errorf("%w: award is already %s",
			ErrAwardAlreadyDecided, award.Status)
	}

	if approve {
		award.Status = AwardApproved
	} else {
		award.Status = AwardRejected
	}
	award.DecidedBy = decidedBy
	award.DecidedAt = time.Now().UTC()
	award.DecisionNote = note
	if err := l.Awards.SetAward(ctx, award); err != nil {
		return Award{}, err
	}
	return award, nil
}

// ImportReceipts stores receipts of covered revenue, reporting how many
// were created and how many were already present. Receipts are keyed by
// their source reference, so re-importing a batch is a no-op by design
// and the operation is safe to retry.
func (l Ledger) ImportReceipts(
	ctx context.Context, receipts []Receipt,
) (imported, duplicates int, err error) {
	for _, r := range receipts {
		switch err := l.Receipts.CreateReceipt(ctx, r); {
		case err == nil:
			imported++
		case errors.Is(err, document.ErrAlreadyExists):
			duplicates++
		default:
			return imported, duplicates,
				fmt.Errorf("receipt %q: %w", r.Source, err)
		}
	}
	return imported, duplicates, nil
}
