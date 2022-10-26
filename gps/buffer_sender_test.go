package gps

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testingSender struct {
	err  error
	send int
}

func (s *testingSender) Send(context.Context, Coordinates) error {
	s.send++
	return s.err
}

func (s *testingSender) Close() error {
	return nil
}

func assertCacheSize(t *testing.T, cache document.DroppableService, size int) {
	it, err := cache.List(context.Background(), nil)
	require.NoError(t, err)

	for it.HasNext() {
		var pos Coordinates
		_ = it.NextTo(&pos)
		size--
	}

	assert.Equal(t, 0, size)
}

func TestBufferSender(t *testing.T) {
	ctx := context.Background()
	t.Run("should send positions via underlying Sender", func(t *testing.T) {
		s := testingSender{err: nil}
		buf := WithBufferFallback(&s, document.NewInMemoryService())
		err := buf.Send(ctx, fixtureCoords)
		require.NoError(t, err)

		assert.Equal(t, 1, s.send)
	})

	t.Run("should buffer positions if underlying Sender fails", func(t *testing.T) {
		s := testingSender{err: errors.New("sup")}
		cache := document.NewInMemoryService()
		buf := WithBufferFallback(&s, cache)

		err := buf.Send(ctx, fixtureCoords)
		require.NoError(t, err)
		assert.Equal(t, 1, s.send)
		assertCacheSize(t, cache, 1)

		pos := fixtureCoords
		pos.UnixTime -= int64(1 * time.Minute)
		err = buf.Send(ctx, pos)
		require.NoError(t, err)
		assert.Equal(t, 2, s.send)
		assertCacheSize(t, cache, 2)

		s.err = nil
		err = buf.Send(ctx, fixtureCoords)
		require.NoError(t, err)
		assert.Equal(t, 5, s.send)
		assertCacheSize(t, cache, 0)
	})
}
