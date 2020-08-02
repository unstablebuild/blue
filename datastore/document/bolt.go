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

type boltStore struct {
	db     *bolt.DB
	collID []byte
}

// NewBolt returns an instance of Service backed by a local, embedded bolt DB.
func NewBolt(dbPath string, collectionID string) (Service, error) {
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
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(collID)
		if err != nil {
			return fmt.Errorf("create collection: %s", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &boltStore{
		db:     db,
		collID: collID,
	}, nil
}

func (s *boltStore) Close() error {
	return s.db.Close()
}

func (s *boltStore) getData(ID string, doc interface{}) (
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

func (s *boltStore) Set(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.set(ctx, ID, doc, false)
}

func (s *boltStore) Create(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.set(ctx, ID, doc, true)
}

func (s *boltStore) set(
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

func (s *boltStore) Update(
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
		b := tx.Bucket([]byte(s.collID))
		return b.Put([]byte(ID), encode(doc, false))
	})
}

func (s *boltStore) Get(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.getData(ID, doc)
}

func (s *boltStore) Delete(
	ctx context.Context, ID string,
) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.collID))
		return b.Delete([]byte(ID))
	})
}

func (s *boltStore) List(ctx context.Context, filters []Filter) (
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
