package gps

import (
	"context"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertIterCount(t *testing.T, it document.Iterator, expected int) []Coordinates {
	var i int
	var ret []Coordinates
	for it.HasNext() {
		var pos Coordinates
		err := it.NextTo(&pos)
		require.NoError(t, err)
		require.NotZero(t, pos)
		i++
		ret = append(ret, pos)
	}
	assert.Equal(t, expected, i)
	require.Len(t, ret, expected)
	return ret
}

func newTestingStore(t *testing.T, opts ...Option) *Store {
	ctx := context.Background()
	b := document.NewInMemoryCache()
	s := NewStore(b, opts...)

	err := s.Receive(ctx, c1)
	require.NoError(t, err)

	err = s.Receive(ctx, c2)
	require.NoError(t, err)

	err = s.Receive(ctx, c3)
	require.NoError(t, err)

	return s
}

func TestStoreReceiverNoOptions(t *testing.T) {
	ctx := context.Background()
	s := newTestingStore(t)

	t.Run("List returns gps data for all devices", func(t *testing.T) {
		it, err := s.List(ctx, time.Time{}, time.Now())
		require.NoError(t, err)
		res := assertIterCount(t, it, 3)
		assert.ElementsMatch(t, res, []Coordinates{c1, c2, c3})
	})

	t.Run("List returns gps data in a left inclusive, right exclusive fashion", func(t *testing.T) {
		it, err := s.List(ctx, first, second)
		require.NoError(t, err)
		assertIterCount(t, it, 1)
	})

	t.Run("List returns empty iterator if range does not contain gps data", func(t *testing.T) {
		it, err := s.List(ctx, time.Now(), time.Now())
		require.NoError(t, err)
		assertIterCount(t, it, 0)
	})

	t.Run("ListDevice returns empty iterator if range does not contain gps data", func(t *testing.T) {
		{
			it, err := s.ListDevice(ctx, deviceID, time.Now(), time.Now())
			require.NoError(t, err)
			assertIterCount(t, it, 0)
		}
		{
			it, err := s.ListDevice(ctx, "blah", first, second)
			require.NoError(t, err)
			assertIterCount(t, it, 0)
		}
	})

	t.Run("ListDevice returns all gps data for a device", func(t *testing.T) {
		it, err := s.ListDevice(ctx, deviceID, time.Time{}, time.Now())
		require.NoError(t, err)
		res := assertIterCount(t, it, 2)
		assert.ElementsMatch(t, []Coordinates{c1, c2}, res)
	})

	t.Run("ListDevice returns gps data in a left inclusive, right exclusive fashion", func(t *testing.T) {
		it, err := s.ListDevice(ctx, deviceID, first, second)
		require.NoError(t, err)
		assertIterCount(t, it, 1)
	})
}

func TestStoreReceiverSampling(t *testing.T) {
	ctx := context.Background()
	s := newTestingStore(t, WithDownSampling(time.Hour))

	err := s.Receive(ctx, c4)
	require.NoError(t, err)

	t.Run("List returns gps downsampled data for all devices", func(t *testing.T) {
		it, err := s.List(ctx, time.Time{}, time.Now())
		require.NoError(t, err)
		res := assertIterCount(t, it, 3)
		assert.ElementsMatch(t, res, []Coordinates{c2, c3, c4})
	})
}

func TestStoreReceiverFixedSize(t *testing.T) {
	ctx := context.Background()
	s := newTestingStore(t, WithFixedSize(1),
		WithDownSampling(time.Minute))

	err := s.Receive(ctx, c4)
	require.NoError(t, err)

	c5 := c3
	c5.UnixTime += 100
	err = s.Receive(ctx, c5)
	require.NoError(t, err)

	t.Run("List returns gps last N records for all devices", func(t *testing.T) {
		it, err := s.List(ctx, time.Time{}, time.Now())
		require.NoError(t, err)
		res := assertIterCount(t, it, 2)
		assert.ElementsMatch(t, res, []Coordinates{c4, c5})
	})
}

func TestStoreReceiverThrottling(t *testing.T) {
	ctx := context.Background()
	s := newTestingStore(t, WithRateLimiting(1*time.Minute),
		WithDownSampling(time.Minute))

	t.Run("List returns last records for all devices", func(t *testing.T) {
		it, err := s.List(ctx, time.Time{}, time.Now())
		require.NoError(t, err)
		res := assertIterCount(t, it, 2)
		assert.ElementsMatch(t, res, []Coordinates{c1, c3})
	})
}
