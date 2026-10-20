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

package ipblock

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document"
)

// NewService returns a List that keeps one document per block in db.
// Every call reads or writes db, and Check reads the whole collection,
// so wherever addresses are checked often, check them through NewCache
// instead.
//
// A lapsed block is ignored but its document is not deleted. On
// Firestore, a TTL policy on the ExpiresAt field deletes them; it is
// null on blocks that never lapse, which a TTL policy leaves alone.
func NewService(db document.Service) List {
	if db == nil {
		panic("ipblock.NewService: db must not be nil")
	}
	return newDocumentList(db, time.Now)
}

type documentList struct {
	db  document.Service
	now func() time.Time
}

func newDocumentList(db document.Service, now func() time.Time) *documentList {
	return &documentList{db: db, now: now}
}

// blockDocument is a Block as stored. Its Go field names name the
// document fields, since document.Service does not let tags rename
// them; Firestore keeps them as they are.
type blockDocument struct {
	Prefix    string
	Reason    string
	BlockedBy string
	BlockedAt time.Time
	// ExpiresAt is nil for blocks that never lapse: a zero timestamp
	// would be in the past, and a TTL policy would delete the block.
	ExpiresAt *time.Time
}

func toDocument(b Block) blockDocument {
	doc := blockDocument{
		Prefix:    b.Prefix.String(),
		Reason:    b.Reason,
		BlockedBy: b.BlockedBy,
		BlockedAt: b.BlockedAt,
	}
	if !b.ExpiresAt.IsZero() {
		doc.ExpiresAt = &b.ExpiresAt
	}
	return doc
}

func fromDocument(doc blockDocument) (Block, error) {
	p, err := netip.ParsePrefix(doc.Prefix)
	if err != nil {
		return Block{}, err
	}
	if p, err = canonical(p); err != nil {
		return Block{}, err
	}
	b := Block{
		Prefix:    p,
		Reason:    doc.Reason,
		BlockedBy: doc.BlockedBy,
		BlockedAt: doc.BlockedAt,
	}
	if doc.ExpiresAt != nil {
		b.ExpiresAt = *doc.ExpiresAt
	}
	return b, nil
}

// documentID is the ID of p's document. Firestore IDs cannot hold a
// slash, so the one separating the prefix length becomes an underscore.
func documentID(p netip.Prefix) string {
	return strings.Replace(p.String(), "/", "_", 1)
}

func (d *documentList) Block(ctx context.Context, b Block) error {
	b, err := prepare(b, d.now())
	if err != nil {
		return err
	}
	if err := d.db.Set(ctx, documentID(b.Prefix), toDocument(b)); err != nil {
		return fmt.Errorf("ipblock: block %s: %w", b.Prefix, err)
	}
	return nil
}

func (d *documentList) Unblock(ctx context.Context, p netip.Prefix) error {
	p, err := canonical(p)
	if err != nil {
		return err
	}
	id := documentID(p)
	var doc blockDocument
	if err := d.db.Get(ctx, id, &doc); errors.Is(err, document.ErrNotFound) {
		return ErrNotBlocked
	} else if err != nil {
		return fmt.Errorf("ipblock: unblock %s: %w", p, err)
	}
	if err := d.db.Delete(ctx, id); err != nil {
		return fmt.Errorf("ipblock: unblock %s: %w", p, err)
	}
	// A lapsed block was not in force, though its document lingered.
	if doc.ExpiresAt != nil && !d.now().Before(*doc.ExpiresAt) {
		return ErrNotBlocked
	}
	return nil
}

func (d *documentList) Check(ctx context.Context, addr netip.Addr) (Block, bool, error) {
	addr, err := canonicalAddr(addr)
	if err != nil {
		return Block{}, false, err
	}
	set, err := d.load(ctx)
	if err != nil {
		return Block{}, false, err
	}
	b, ok := set.lookup(addr, d.now())
	return b, ok, nil
}

func (d *documentList) List(ctx context.Context) ([]Block, error) {
	set, err := d.load(ctx)
	if err != nil {
		return nil, err
	}
	return set.list(d.now()), nil
}

// load reads every block in the collection. A document that does not
// hold a valid block is skipped rather than failing the load: one
// hand-edited document must not lift every other block.
func (d *documentList) load(ctx context.Context) (*blockSet, error) {
	it, err := d.db.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("ipblock: list blocks: %w", err)
	}
	defer func() { _ = it.Close() }()
	var blocks []Block
	for it.HasNext() {
		var doc blockDocument
		if err := it.NextTo(&doc); err != nil {
			return nil, fmt.Errorf("ipblock: list blocks: %w", err)
		}
		b, err := fromDocument(doc)
		if err != nil {
			log.WithError(err).WithField("prefix", doc.Prefix).
				Warn("ipblock: skipping invalid block document")
			continue
		}
		blocks = append(blocks, b)
	}
	return newBlockSet(blocks), nil
}
