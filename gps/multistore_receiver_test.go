package gps

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newEmptyMultiStore(t *testing.T) *MultiStore {
	return &MultiStore{
		cache:       NewCache(document.NewInMemoryCache()),
		dayStore:    NewStore(document.NewInMemoryCache(), dayStoreOpts...),
		hourlyStore: NewStore(document.NewInMemoryCache(), hourStoreOpts...),
		minuteStore: NewStore(document.NewInMemoryCache(), minuteStoreOpts...),
	}
}

func assertDeviceOneRecord(t *testing.T, store *Store) {
	ctx := context.Background()
	it, err := store.ListDevice(ctx, deviceID, time.Time{}, time.Now())
	require.NoError(t, err)
	assertIterCount(t, it, 1)
}

// for store backend IDs are only important for writes, so we can create
// document with a randon UUID to test reads
func createForStoreBackend(t *testing.T, pos Coordinates, backends []document.Service) {
	ctx := context.Background()

	for _, b := range backends {
		require.NoError(t, b.Create(ctx, uuid.New().String(), pos))
	}
}

func createForCacheBackend(t *testing.T, pos Coordinates, b document.Service) {
	ctx := context.Background()
	require.NoError(t, b.Create(ctx, pos.DeviceID, pos))
}

func newFullMultiStore(t *testing.T) *MultiStore {
	m := newEmptyMultiStore(t)
	b1, b2, b3, b4 := m.cache.backend, m.dayStore.backend,
		m.hourlyStore.backend, m.minuteStore.backend

	createForCacheBackend(t, c1, b1)
	createForCacheBackend(t, c3, b1)
	createForStoreBackend(t, c1, []document.Service{b2, b3, b4})
	createForStoreBackend(t, c2, []document.Service{b2, b3, b4})
	createForStoreBackend(t, c3, []document.Service{b3, b4})
	createForStoreBackend(t, c4, []document.Service{b3})

	return m
}

func TestMultiStoreReceive(t *testing.T) {
	ctx := context.Background()

	t.Run("stores gps position in underlying stores", func(t *testing.T) {
		m := newEmptyMultiStore(t)
		err := m.Receive(ctx, c1)
		require.NoError(t, err)

		_, err = m.cache.Get(ctx, c1.DeviceID)
		require.NoError(t, err) // not found would return err

		assertDeviceOneRecord(t, m.dayStore)
		assertDeviceOneRecord(t, m.minuteStore)
		assertDeviceOneRecord(t, m.hourlyStore)
	})

	t.Run("bubbles up storage errors", func(t *testing.T) {
		m := newEmptyMultiStore(t)
		m.cache.backend = &failingDocService{err: errors.New("Karen sux")}

		err := m.Receive(ctx, c1)
		assert.Error(t, err)
	})
}

func TestMultiStoreListDevice(t *testing.T) {
	ctx := context.Background()
	m := newFullMultiStore(t)

	t.Run("lists all coordinates of a device", func(t *testing.T) {
		it, err := m.ListDevice(ctx, c3.DeviceID, time.Time{}, time.Now())
		require.NoError(t, err)
		assertIterCount(t, it, 1)
	})

	t.Run("returns ErrNotFound if deviceID has not been seen", func(t *testing.T) {
		_, err := m.ListDevice(ctx, "sup", time.Time{}, time.Now())
		require.Equal(t, document.ErrNotFound, err)
	})

	t.Run("dedupes duplicate coordinates", func(t *testing.T) {
		it, err := m.ListDevice(ctx, c1.DeviceID, time.Time{}, time.Now())
		require.NoError(t, err)
		assertIterCount(t, it, 3)
	})
}

func TestMultiStoreGetDevice(t *testing.T) {
	ctx := context.Background()
	m := newFullMultiStore(t)

	t.Run("gets latest coords of a device", func(t *testing.T) {
		pos, err := m.GetDevice(ctx, c1.DeviceID)
		require.NoError(t, err)
		assert.Equal(t, c1, pos)
	})

	t.Run("returns ErrNotFound if deviceID has not been seen", func(t *testing.T) {
		_, err := m.GetDevice(ctx, "sup")
		require.Equal(t, document.ErrNotFound, err)
	})
}

func TestMultiStoreGetAll(t *testing.T) {
	ctx := context.Background()
	m := newFullMultiStore(t)

	t.Run("gets latest coords of a device", func(t *testing.T) {
		pos, err := m.GetAll(ctx)
		require.NoError(t, err)
		assert.Len(t, pos, 2)
		assert.ElementsMatch(t, []Coordinates{c1, c3}, pos)
	})
}
