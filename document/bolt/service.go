// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package bolt

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	bolt "go.etcd.io/bbolt"
)

const (
	defaultBoltTimeout = 10 * time.Second
)

var (
	// only one instance of db per path can be instantiated
	// and often we want to instante multiple bolt.Store's
	// in the same db path, one per collection.
	mu      sync.Mutex
	dbs     = make(map[string]*sharedDB)
	options = bolt.Options{Timeout: defaultBoltTimeout}
)

// sharedDB tracks how many live Stores reference a single *bolt.DB so the
// last Close releases the underlying handle. Multiple Stores (one per
// collection, or one per consumer of the same path) share a DB by design,
// so closing one must not evict the handle the others still use.
type sharedDB struct {
	db   *bolt.DB
	refs int
}

// Store implements a document.Service backed by a local, embedded bolt DB.
// It additionally provides a method to efficiently delete all
// contents of a collection: DeleteAll.
type Store struct {
	marshaler docmarshal.Marshaler
	db        *bolt.DB
	dbPath    string
	collID    []byte
}

// New allocates store for a new Store and initializes it with the given
// dbPath and collectionID.
func New(dbPath string, collectionID string) (*Store, error) {
	return NewWithMarshaler(dbPath, collectionID, docbson.Marshaler())
}

// NewWithMarshaler allocates a new Store using the given marshaler for
// encoding stored documents. Callers that need the storage marshaler to
// match the wire marshaler used elsewhere (for type-strict CAS preconditions
// to round-trip identically) should pass the wire marshaler here.
func NewWithMarshaler(
	dbPath string, collectionID string, marshaler docmarshal.Marshaler,
) (*Store, error) {
	mu.Lock()
	if dbs[dbPath] == nil {
		mu.Unlock()
		db, err := bolt.Open(dbPath, 0600, &options)
		mu.Lock()
		if err != nil {
			err = fmt.Errorf("could not open DB at path %s: %v", dbPath, err)
			mu.Unlock()
			return nil, err
		}
		// Another caller may have opened and cached the same path while
		// this goroutine had the lock released for bolt.Open; close the
		// redundant handle and reuse the cached one.
		if dbs[dbPath] == nil {
			dbs[dbPath] = &sharedDB{db: db}
		} else {
			_ = db.Close()
		}
	}

	shared := dbs[dbPath]
	shared.refs++
	db := shared.db
	mu.Unlock()

	collID := []byte(collectionID)
	s := &Store{
		db:     db,
		dbPath: dbPath,
		collID: collID,
	}
	err := s.createBucketIfNotExists(context.Background())
	if err != nil {
		_ = s.Close()
		return nil, err
	}
	s.marshaler = marshaler
	return s, nil
}

func (s *Store) createBucketIfNotExists(ctx context.Context) error {
	return s.update(ctx, func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(s.collID)
		if err != nil {
			return fmt.Errorf("create collection: %s", err)
		}
		return nil
	})
}

// update runs fn in a read-write transaction whose commit durability
// honors ctx (see ContextWithNoSync). The flag is assigned inside the
// transaction: bbolt reads db.NoSync only while committing (tx.write,
// tx.writeMeta), under the writer lock the transaction already holds,
// so writing it here orders every flip against every commit and each
// commit uses exactly the durability its own operation requested.
func (s *Store) update(ctx context.Context, fn func(tx *bolt.Tx) error) error {
	noSync := NoSyncRequested(ctx)
	return s.db.Update(func(tx *bolt.Tx) error {
		s.db.NoSync = noSync
		return fn(tx)
	})
}

// Close releases this Store's reference to the shared *bolt.DB. The
// underlying handle is closed (and evicted from the package-level dbs
// cache) only when the last Store referencing the path closes, so sibling
// Stores opened against the same dbPath keep working.
func (s *Store) Close() error {
	mu.Lock()
	shared, ok := dbs[s.dbPath]
	if !ok || shared.db != s.db {
		mu.Unlock()
		return nil
	}
	shared.refs--
	if shared.refs > 0 {
		mu.Unlock()
		return nil
	}
	delete(dbs, s.dbPath)
	mu.Unlock()
	return s.db.Close()
}

func (s *Store) getData(ID string, doc any) (
	err error,
) {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		data := b.Get([]byte(ID))

		if len(data) == 0 {
			return document.ErrNotFound
		}

		return document.SafeDecode(s.marshaler, doc, data)
	})
}

// Set satisfies document.Service.
func (s *Store) Set(
	ctx context.Context, ID string, doc any,
) error {
	return s.set(ctx, ID, doc, false)
}

// Create satisfies document.Service.
func (s *Store) Create(
	ctx context.Context, ID string, doc any,
) error {
	return s.set(ctx, ID, doc, true)
}

func (s *Store) set(
	ctx context.Context, ID string, doc any,
	errAlreadyExists bool,
) error {
	if doc == nil {
		panic("invalid nil data argument to Create")
	}
	doc, err := document.DerefCreateValue(reflect.ValueOf(doc))
	if err != nil {
		return err
	}

	return s.update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		key := []byte(ID)
		if errAlreadyExists && len(b.Get(key)) != 0 {
			return document.ErrAlreadyExists
		}
		return b.Put(key, document.Encode(s.marshaler, doc, true))
	})
}

// Update satisfies document.Service.
func (s *Store) Update(
	ctx context.Context, ID string, updates []document.Update,
	preconds ...document.Precondition,
) error {
	if len(updates) == 0 {
		panic("Update: no paths to update")
	}

	return s.update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		data := b.Get([]byte(ID))

		if len(data) == 0 {
			return document.ErrNotFound
		}

		var doc map[string]any
		err := document.SafeDecode(s.marshaler, &doc, data)
		if err != nil {
			return err
		}

		err = document.UpdateProto(s.marshaler, updates, doc, preconds...)
		if err != nil {
			return err
		}

		return b.Put([]byte(ID), document.Encode(s.marshaler, doc, false))
	})
}

// Get satisfies document.Service.
func (s *Store) Get(
	ctx context.Context, ID string, doc any,
) error {
	return s.getData(ID, doc)
}

// Delete satisfies document.Service.
func (s *Store) Delete(
	ctx context.Context, ID string,
) error {
	return s.update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		return b.Delete([]byte(ID))
	})
}

// List satisfies document.Service.
func (s *Store) List(ctx context.Context, filters []document.Filter) (
	document.Iterator, error,
) {
	iter := document.NewListIterator(s.marshaler)
	err := s.db.View(func(tx *bolt.Tx) error {
		// NOTE: this buffers all results in memory.
		// We should paginate results by creating a cursor
		// every time NextTo exhausts a certain number of buffered
		// documents.
		b := tx.Bucket(s.collID)
		return b.ForEach(func(key []byte, value []byte) error {
			iter.Extend(filters, value)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return iter, nil
}

// Drop satisfies document.DroppableService.
func (s *Store) Drop(ctx context.Context) error {
	err := s.update(ctx, func(tx *bolt.Tx) error {
		return tx.DeleteBucket(s.collID)
	})
	if err != nil {
		return err
	}

	// if this fails we're screwed because the rest of operations will panic..
	err = s.createBucketIfNotExists(ctx)
	if err != nil {
		panic(fmt.Sprintf("critical error: failed to recreate collection: %v", err))
	}

	return nil
}

// ApplyBatch satisfies document.BatchWriter. Every op is applied in a
// single read-write transaction, so a batch costs one commit — and one
// fsync, unless ctx requests otherwise — instead of one per operation.
func (s *Store) ApplyBatch(
	ctx context.Context, ops []document.BatchOp,
) ([]document.BatchOpResult, error) {
	results := make([]document.BatchOpResult, len(ops))
	err := s.update(ctx, func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		for i, op := range ops {
			err := s.applyBatchOp(b, op)
			switch {
			case errors.Is(err, document.ErrAlreadyExists),
				errors.Is(err, document.ErrNotFound),
				errors.Is(err, document.ErrPreconditionFailed):
				results[i].Err = err
			case err != nil:
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Store) applyBatchOp(b *bolt.Bucket, op document.BatchOp) error {
	key := []byte(op.ID)
	switch op.Type {
	case document.BatchDelete:
		return b.Delete(key)
	case document.BatchCreate, document.BatchSet:
		if op.Doc == nil {
			return fmt.Errorf("batch: nil document for %q", op.ID)
		}
		doc, err := document.DerefCreateValue(reflect.ValueOf(op.Doc))
		if err != nil {
			return err
		}
		if op.Type == document.BatchCreate && len(b.Get(key)) != 0 {
			return document.ErrAlreadyExists
		}
		return b.Put(key, document.Encode(s.marshaler, doc, true))
	case document.BatchUpdate:
		if len(op.Updates) == 0 {
			return fmt.Errorf("batch: no paths to update for %q", op.ID)
		}
		data := b.Get(key)
		if len(data) == 0 {
			return document.ErrNotFound
		}
		var doc map[string]any
		if err := document.SafeDecode(s.marshaler, &doc, data); err != nil {
			return err
		}
		err := document.UpdateProto(
			s.marshaler, op.Updates, doc, op.Preconditions...)
		if err != nil {
			return err
		}
		return b.Put(key, document.Encode(s.marshaler, doc, false))
	default:
		return fmt.Errorf("batch: unknown operation type %d", op.Type)
	}
}
