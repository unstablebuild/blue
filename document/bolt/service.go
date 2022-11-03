package bolt

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/ernestrc/blue/document"
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
	dbs     = make(map[string]*bolt.DB)
	options = bolt.Options{Timeout: defaultBoltTimeout}
)

// Store implements a document.Service backed by a local, embedded bolt DB.
// It additionally provides a method to efficiently delete all
// contents of a collection: DeleteAll.
type Store struct {
	db     *bolt.DB
	collID []byte
}

// New allocates store for a new Store and initializes it with the given
// dbPath and collectionID.
func New(dbPath string, collectionID string) (*Store, error) {
	mu.Lock()
	defer mu.Unlock()
	if dbs[dbPath] == nil {
		mu.Unlock()
		db, err := bolt.Open(dbPath, 0600, &options)
		mu.Lock()
		if err != nil {
			err = fmt.Errorf("Could not open DB at path %s: %v", dbPath, err)
			return nil, err
		}
		dbs[dbPath] = db
	}

	db := dbs[dbPath]

	collID := []byte(collectionID)
	s := &Store{
		db:     db,
		collID: collID,
	}
	err := s.createBucketIfNotExists()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) createBucketIfNotExists() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(s.collID)
		if err != nil {
			return fmt.Errorf("create collection: %s", err)
		}
		return nil
	})
}

// Close closes all resources associated with this Store .
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) getData(ID string, doc interface{}) (
	err error,
) {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		data := b.Get([]byte(ID))

		if len(data) == 0 {
			return document.ErrNotFound
		}

		return document.SafeDecode(doc, data)
	})
}

// Set satisfies document.Service.
func (s *Store) Set(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.set(ctx, ID, doc, false)
}

// Create satisfies document.Service.
func (s *Store) Create(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.set(ctx, ID, doc, true)
}

func (s *Store) set(
	ctx context.Context, ID string, doc interface{},
	errAlreadyExists bool,
) error {
	if doc == nil {
		panic("invalid nil data argument to Create")
	}
	doc, err := document.DerefCreateValue(reflect.ValueOf(doc))
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		key := []byte(ID)
		if errAlreadyExists && len(b.Get(key)) != 0 {
			return document.ErrAlreadyExists
		}
		return b.Put(key, document.Encode(doc, true))
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

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		data := b.Get([]byte(ID))

		if len(data) == 0 {
			return document.ErrNotFound
		}

		var doc map[string]interface{}
		err := document.SafeDecode(&doc, data)
		if err != nil {
			return err
		}

		err = document.UpdateProto(true, updates, doc, preconds...)
		if err != nil {
			return err
		}

		return b.Put([]byte(ID), document.Encode(doc, false))
	})
}

// Get satisfies document.Service.
func (s *Store) Get(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.getData(ID, doc)
}

// Delete satisfies document.Service.
func (s *Store) Delete(
	ctx context.Context, ID string,
) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		return b.Delete([]byte(ID))
	})
}

// List satisfies document.Service.
func (s *Store) List(ctx context.Context, filters []document.Filter) (
	document.Iterator, error,
) {
	iter := document.NewListIterator()
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
	err := s.db.Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket(s.collID)
	})
	if err != nil {
		return err
	}

	// if this fails we're screwed because the rest of operations will panic..
	err = s.createBucketIfNotExists()
	if err != nil {
		panic(fmt.Sprintf("critical error: failed to recreate collection: %v", err))
	}

	return nil
}
