package document

import (
	"context"
	"fmt"
	"reflect"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	defaultBoltTimeout = 10 * time.Second
)

var (
	// only one instance of db per path can be instantiated
	dbs     = make(map[string]*bolt.DB)
	options = bolt.Options{Timeout: defaultBoltTimeout}
)

// BoltStore implements a document.Service backed by a local, embedded bolt DB.
// It additionally provides a method to efficiently delete all
// contents of a collection: DeleteAll.
type BoltStore struct {
	db     *bolt.DB
	collID []byte
}

// NewBolt allocates store for a new BoltStore and initializes it with the given
// dbPath and collectionID.
func NewBolt(dbPath string, collectionID string) (*BoltStore, error) {
	if dbs[dbPath] == nil {
		db, err := bolt.Open(dbPath, 0600, &options)
		if err != nil {
			err = fmt.Errorf("Could not open DB at path %s: %v", dbPath, err)
			return nil, err
		}
		dbs[dbPath] = db
	}

	db := dbs[dbPath]

	collID := []byte(collectionID)
	s := &BoltStore{
		db:     db,
		collID: collID,
	}
	err := s.createBucketIfNotExists()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *BoltStore) createBucketIfNotExists() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(s.collID)
		if err != nil {
			return fmt.Errorf("create collection: %s", err)
		}
		return nil
	})
}

// Close closes all resources associated with this BoltStore.
func (s *BoltStore) Close() error {
	return s.db.Close()
}

func (s *BoltStore) getData(ID string, doc interface{}) (
	err error,
) {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		data := b.Get([]byte(ID))

		if len(data) == 0 {
			return ErrNotFound
		}

		return safeDecode(doc, data)
	})
}

// Set satisfies document.Service.
func (s *BoltStore) Set(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.set(ctx, ID, doc, false)
}

// Create satisfies document.Service.
func (s *BoltStore) Create(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.set(ctx, ID, doc, true)
}

func (s *BoltStore) set(
	ctx context.Context, ID string, doc interface{},
	errAlreadyExists bool,
) error {
	if doc == nil {
		panic("invalid nil data argument to Create")
	}
	doc, err := derefCreateValue(reflect.ValueOf(doc))
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(true)
	if err != nil {
		return err
	}

	defer func() {
		err := recover()
		if err != nil {
			_ = tx.Rollback()
			panic(err)
		}
	}()

	b := tx.Bucket(s.collID)
	key := []byte(ID)

	if errAlreadyExists && len(b.Get(key)) != 0 {
		_ = tx.Rollback()
		return ErrAlreadyExists
	}

	err = b.Put(key, encode(doc, true))
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Update satisfies document.Service.
func (s *BoltStore) Update(
	ctx context.Context, ID string, updates []Update,
) error {
	if len(updates) == 0 {
		panic("Update: no paths to update")
	}

	var doc map[string]interface{}
	err := s.getData(ID, &doc)
	if err != nil {
		return err
	}

	updateProto(updates, doc)

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		return b.Put([]byte(ID), encode(doc, false))
	})
}

// Get satisfies document.Service.
func (s *BoltStore) Get(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.getData(ID, doc)
}

// Delete satisfies document.Service.
func (s *BoltStore) Delete(
	ctx context.Context, ID string,
) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.collID)
		return b.Delete([]byte(ID))
	})
}

// List satisfies document.Service.
func (s *BoltStore) List(ctx context.Context, filters []Filter) (
	Iterator, error,
) {
	iter := &listIterator{docs: make([][]byte, 0)}
	err := s.db.View(func(tx *bolt.Tx) error {
		// NOTE: this buffers all results in memory.
		// We should paginate results by creating a cursor
		// every time NextTo exhausts a certain number of buffered
		// documents.
		b := tx.Bucket(s.collID)
		return b.ForEach(func(key []byte, value []byte) error {
			iter.maybeExtend(filters, value)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return iter, nil
}

// DeleteAll efficiently deletes all documents in the collection.
func (s *BoltStore) DeleteAll(ctx context.Context) error {
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
