package gps

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/bolt"
	"github.com/unstablebuild/blue/document/firestore"
)

var (
	dayStoreOpts = []Option{
		WithRateLimiting(4 * time.Hour),
		WithDownSampling(24 * time.Hour),
		WithLoggingLabel("DayStore"),
	}

	hourStoreOpts = []Option{
		WithRateLimiting(30 * time.Minute),
		WithDownSampling(time.Hour),
		WithFixedSize(31 * 24),
		WithLoggingLabel("HourStore"),
	}

	minuteStoreOpts = []Option{
		WithDownSampling(time.Minute),
		WithFixedSize(24 * 60),
		WithLoggingLabel("MinuteStore"),
	}

	cacheOpts = []Option{
		WithLoggingLabel("BoltCache"),
	}
)

// MultiStoreConfig is the configuration required to instantiate a new
// MultiStore. See NewMultiStore for more details.
type MultiStoreConfig struct {
	CacheCollection  string
	MinuteCollection string
	DayCollection    string
	HourCollection   string

	Firestore struct {
		ProjectID string
		CredsFile string
	}

	Bolt struct {
		DBPath string
	}
}

// DefaultMultiStoreConfig returns a MultiStoreConfig with the
// default collection names.
func DefaultMultiStoreConfig() MultiStoreConfig {
	return MultiStoreConfig{
		CacheCollection:  "gps-cache",
		MinuteCollection: "gps-minute",
		DayCollection:    "gps-day",
		HourCollection:   "gps-hour",
	}
}

// MultiStore provides satisfies Receiver by persisting the data
// with different downsampling, throttling and fixed-size database
// policies to minimize costs and maximize data retention.
type MultiStore struct {
	cache       *Cache
	dayStore    *Store
	hourlyStore *Store
	minuteStore *Store
}

// NewMultiStore allocates storage for a MultiStore and initializes
// it with the given config.
func NewMultiStore(config MultiStoreConfig) (m *MultiStore, err error) {
	m = new(MultiStore)
	var backend document.Service

	backend, err = bolt.New(config.Bolt.DBPath, config.CacheCollection)
	if err != nil {
		return
	}
	m.cache = NewCache(backend, cacheOpts...)

	backend, err = bolt.New(config.Bolt.DBPath, config.MinuteCollection)
	if err != nil {
		return
	}
	m.minuteStore = NewStore(backend, minuteStoreOpts...)

	backend, err = firestore.New(config.Firestore.ProjectID,
		config.DayCollection, config.Firestore.CredsFile)
	if err != nil {
		return
	}
	m.dayStore = NewStore(backend, dayStoreOpts...)

	backend, err = firestore.New(config.Firestore.ProjectID,
		config.HourCollection, config.Firestore.CredsFile)
	if err != nil {
		return
	}
	m.hourlyStore = NewStore(backend, hourStoreOpts...)

	return
}

// Receive satisfies Reicever.
func (m *MultiStore) Receive(ctx context.Context, pos Coordinates) error {
	const writes = 4
	ch := make(chan error)

	go func(ctx context.Context, pos Coordinates) {
		ch <- m.cache.Receive(ctx, pos)
	}(ctx, pos)

	go func(ctx context.Context, pos Coordinates) {
		ch <- m.dayStore.Receive(ctx, pos)
	}(ctx, pos)

	go func(ctx context.Context, pos Coordinates) {
		ch <- m.hourlyStore.Receive(ctx, pos)
	}(ctx, pos)

	go func(ctx context.Context, pos Coordinates) {
		ch <- m.minuteStore.Receive(ctx, pos)
	}(ctx, pos)

	var errs []error
	for i := 0; i < writes; i++ {
		select {
		case err := <-ch:
			if err != nil {
				errs = append(errs, err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("Could not persist gps coordinates %+v: %+v", pos, errs)
}

// GetDevice gets the latest known position of a given deviceID.
func (m *MultiStore) GetDevice(ctx context.Context, deviceID string) (
	Coordinates, error,
) {
	return m.cache.Get(ctx, deviceID)
}

// GetAll retrieves the last stored Coordinates of all known devices.
func (m *MultiStore) GetAll(ctx context.Context) ([]Coordinates, error) {
	return m.cache.GetAll(ctx)
}

type multiStoreIter struct {
	dedupe map[int64]struct{}
	iter   []document.Iterator
	ready  []Coordinates

	next    Coordinates
	nextErr error
}

func (m *multiStoreIter) hasNext() bool {
	if len(m.ready) != 0 {
		return true
	}
	for len(m.iter) != 0 {
		if m.iter[0].HasNext() {
			return true
		}
		m.iter = m.iter[1:]
	}
	return false
}
func (m *multiStoreIter) nextTo(pos *Coordinates) error {
	if len(m.ready) != 0 {
		*pos = m.ready[0]
		m.ready = m.ready[1:]
		return nil
	}

	if len(m.iter) == 0 {
		panic("HasNext should be called before NextTo")
	}

	return m.iter[0].NextTo(pos)
}

func (m *multiStoreIter) HasNext() bool {
	var candidate Coordinates
	for m.hasNext() {
		m.nextErr = m.nextTo(&candidate)
		if m.nextErr != nil {
			return true
		}
		_, seen := m.dedupe[candidate.UnixTime]
		if !seen {
			m.dedupe[candidate.UnixTime] = struct{}{}
			m.next = candidate
			return true
		}
	}
	return false
}

func (m *multiStoreIter) NextTo(doc interface{}) error {
	if m.nextErr != nil {
		return m.nextErr
	}

	ptr, ok := doc.(*Coordinates)
	if !ok {
		panic("expecting *Coordinates as iterator receiver")
	}

	*ptr = m.next
	return nil
}

func (m *multiStoreIter) Close() error {
	var err error
	for _, it := range m.iter {
		itErr := it.Close()
		if itErr != nil {
			err = itErr
		}
	}
	return err
}

// ListDevice returns an iterator that returns all known positions of
// a device between from and to.
func (m *MultiStore) ListDevice(
	ctx context.Context, deviceID string, from, to time.Time,
) (document.Iterator, error) {
	const reads = 4
	const cached = 1
	const iterators = reads - cached

	errs := make([]error, reads)
	it := &multiStoreIter{
		dedupe: make(map[int64]struct{}),
		iter:   make([]document.Iterator, iterators),
		ready:  make([]Coordinates, cached),
	}

	var wg sync.WaitGroup
	wg.Add(reads)

	go func() {
		defer wg.Done()
		it.iter[0], errs[0] = m.minuteStore.ListDevice(ctx, deviceID, from, to)
	}()

	go func() {
		defer wg.Done()
		it.iter[1], errs[1] = m.hourlyStore.ListDevice(ctx, deviceID, from, to)
	}()

	go func() {
		defer wg.Done()
		it.iter[2], errs[2] = m.dayStore.ListDevice(ctx, deviceID, from, to)
	}()

	go func() {
		defer wg.Done()
		it.ready[0], errs[3] = m.cache.Get(ctx, deviceID)
	}()

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	return it, nil
}
