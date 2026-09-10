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

	"github.com/google/uuid"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
)

// The document-backed stores below persist each domain type in its own
// document.Service (one collection per type). They work with any
// document.Service implementation: in-memory or bolt in tests, a cloud
// document store in production.

func equalFilter(path []string, value any) document.Filter {
	return document.Filter{
		Field: document.Field{FieldPath: path, Value: value},
		Op:    document.OpEqual,
	}
}

func listDocuments[T any](
	ctx context.Context, db document.Service, filters []document.Filter,
) (iterator.Iterator[T], error) {
	it, err := db.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("document.Service.List: %v", err)
	}
	return iterator.FromDocumentIterator[T](it), nil
}

type documentProgramStore struct {
	db document.Service
}

// NewDocumentProgramStore returns a ProgramStore backed by db.
func NewDocumentProgramStore(db document.Service) ProgramStore {
	return &documentProgramStore{db: db}
}

func (s *documentProgramStore) PutProgram(
	ctx context.Context, p Program,
) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return s.db.Set(ctx, p.Version, p)
}

func (s *documentProgramStore) GetProgram(
	ctx context.Context, version string,
) (Program, error) {
	var p Program
	err := s.db.Get(ctx, version, &p)
	return p, err
}

func (s *documentProgramStore) ProgramFor(
	ctx context.Context, m Month,
) (Program, error) {
	it, err := s.ListPrograms(ctx)
	if err != nil {
		return Program{}, err
	}
	programs, err := iterator.ToSlice(ctx, it)
	if err != nil {
		return Program{}, err
	}
	var best Program
	var found bool
	for _, p := range programs {
		if m.Before(p.EffectiveFrom) {
			continue
		}
		if !found || best.EffectiveFrom.Before(p.EffectiveFrom) {
			best = p
			found = true
		}
	}
	if !found {
		return Program{}, document.ErrNotFound
	}
	return best, nil
}

func (s *documentProgramStore) ListPrograms(
	ctx context.Context,
) (iterator.Iterator[Program], error) {
	return listDocuments[Program](ctx, s.db, nil)
}

type documentParticipantStore struct {
	db document.Service
}

// NewDocumentParticipantStore returns a ParticipantStore backed by db.
func NewDocumentParticipantStore(db document.Service) ParticipantStore {
	return &documentParticipantStore{db: db}
}

func (s *documentParticipantStore) CreateParticipant(
	ctx context.Context, p Participant,
) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return s.db.Create(ctx, p.Account, p)
}

func (s *documentParticipantStore) GetParticipant(
	ctx context.Context, account string,
) (Participant, error) {
	var p Participant
	err := s.db.Get(ctx, account, &p)
	return p, err
}

func (s *documentParticipantStore) SetParticipant(
	ctx context.Context, p Participant,
) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return s.db.Set(ctx, p.Account, p)
}

func (s *documentParticipantStore) ListParticipants(
	ctx context.Context,
) (iterator.Iterator[Participant], error) {
	return listDocuments[Participant](ctx, s.db, nil)
}

type documentAwardStore struct {
	db document.Service
}

// NewDocumentAwardStore returns an AwardStore backed by db.
func NewDocumentAwardStore(db document.Service) AwardStore {
	return &documentAwardStore{db: db}
}

func (s *documentAwardStore) CreateAward(
	ctx context.Context, a Award,
) (string, error) {
	if a.ID != "" {
		return "", errors.New("invalid award: ID is assigned by the store")
	}
	a.ID = uuid.NewString()
	if err := a.Validate(); err != nil {
		return "", err
	}
	if err := s.db.Create(ctx, a.ID, a); err != nil {
		return "", err
	}
	return a.ID, nil
}

func (s *documentAwardStore) GetAward(
	ctx context.Context, id string,
) (Award, error) {
	var a Award
	err := s.db.Get(ctx, id, &a)
	return a, err
}

func (s *documentAwardStore) SetAward(ctx context.Context, a Award) error {
	if a.ID == "" {
		return errors.New("invalid award: missing ID")
	}
	if err := a.Validate(); err != nil {
		return err
	}
	return s.db.Set(ctx, a.ID, a)
}

func (s *documentAwardStore) ListAwards(
	ctx context.Context, status AwardStatus,
) (iterator.Iterator[Award], error) {
	var filters []document.Filter
	if status != "" {
		filters = append(filters,
			equalFilter([]string{"Status"}, string(status)))
	}
	return listDocuments[Award](ctx, s.db, filters)
}

type documentReceiptStore struct {
	db document.Service
}

// NewDocumentReceiptStore returns a ReceiptStore backed by db.
func NewDocumentReceiptStore(db document.Service) ReceiptStore {
	return &documentReceiptStore{db: db}
}

func (s *documentReceiptStore) CreateReceipt(
	ctx context.Context, r Receipt,
) error {
	if err := r.Validate(); err != nil {
		return err
	}
	return s.db.Create(ctx, r.Source, r)
}

func (s *documentReceiptStore) GetReceipt(
	ctx context.Context, source string,
) (Receipt, error) {
	var r Receipt
	err := s.db.Get(ctx, source, &r)
	return r, err
}

func (s *documentReceiptStore) ListMonthReceipts(
	ctx context.Context, m Month,
) (iterator.Iterator[Receipt], error) {
	filters := []document.Filter{
		equalFilter([]string{"Month"}, string(m)),
	}
	return listDocuments[Receipt](ctx, s.db, filters)
}

type documentRoundStore struct {
	db document.Service
}

// NewDocumentRoundStore returns a RoundStore backed by db.
func NewDocumentRoundStore(db document.Service) RoundStore {
	return &documentRoundStore{db: db}
}

func (s *documentRoundStore) CreateRound(
	ctx context.Context, r Round,
) error {
	if err := r.Month.Validate(); err != nil {
		return fmt.Errorf("invalid round: %v", err)
	}
	return s.db.Create(ctx, string(r.Month), r)
}

func (s *documentRoundStore) GetRound(
	ctx context.Context, m Month,
) (Round, error) {
	var r Round
	err := s.db.Get(ctx, string(m), &r)
	return r, err
}

func (s *documentRoundStore) ListRounds(
	ctx context.Context,
) (iterator.Iterator[Round], error) {
	return listDocuments[Round](ctx, s.db, nil)
}

type documentObligationStore struct {
	db document.Service
}

// NewDocumentObligationStore returns an ObligationStore backed by db.
func NewDocumentObligationStore(db document.Service) ObligationStore {
	return &documentObligationStore{db: db}
}

func (s *documentObligationStore) CreateObligation(
	ctx context.Context, o Obligation,
) error {
	if err := o.Month.Validate(); err != nil {
		return fmt.Errorf("invalid obligation: %v", err)
	}
	if o.Account == "" {
		return errors.New("invalid obligation: missing account")
	}
	return s.db.Create(ctx, o.ID(), o)
}

func (s *documentObligationStore) GetObligation(
	ctx context.Context, m Month, account string,
) (Obligation, error) {
	var o Obligation
	err := s.db.Get(ctx, ObligationID(m, account), &o)
	return o, err
}

func (s *documentObligationStore) SetObligation(
	ctx context.Context, o Obligation,
) error {
	if o.Account == "" {
		return errors.New("invalid obligation: missing account")
	}
	return s.db.Set(ctx, o.ID(), o)
}

func (s *documentObligationStore) ListAccountObligations(
	ctx context.Context, account string,
) (iterator.Iterator[Obligation], error) {
	filters := []document.Filter{
		equalFilter([]string{"Account"}, account),
	}
	return listDocuments[Obligation](ctx, s.db, filters)
}

func (s *documentObligationStore) ListStateObligations(
	ctx context.Context, state ObligationState,
) (iterator.Iterator[Obligation], error) {
	filters := []document.Filter{
		equalFilter([]string{"State"}, string(state)),
	}
	return listDocuments[Obligation](ctx, s.db, filters)
}
