package gps

import (
	"context"
	"testing"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheReceiver(t *testing.T) {
	coords := fixtureCoords
	b := document.NewInMemoryCache()
	c := NewCache(b)

	ctx := context.Background()
	t.Run("GetAll returns an empty slice, rather than nil when there are no devices stored", func(t *testing.T) {
		allPos, err := c.GetAll(ctx)
		require.NoError(t, err)
		assert.Len(t, allPos, 0)
		assert.Equal(t, []Coordinates{}, allPos)
	})

	err := c.Receive(ctx, coords)
	require.NoError(t, err)

	t.Run("Get", func(t *testing.T) {
		pos, err := c.Get(ctx, deviceID)
		require.NoError(t, err)
		assert.Equal(t, coords, pos)
	})

	t.Run("GetAll returns one dp for each known device", func(t *testing.T) {
		pos := fixtureCoords
		allPos, err := c.GetAll(ctx)
		require.NoError(t, err)
		assert.Len(t, allPos, 1)
		assert.Equal(t, pos, allPos[0])

		pos2 := pos
		pos2.DeviceID = "jfkewkjl"
		err = c.Receive(ctx, pos2)
		require.NoError(t, err)

		allPos, err = c.GetAll(ctx)
		require.NoError(t, err)
		assert.Len(t, allPos, 2)
		assert.ElementsMatch(t, []Coordinates{pos, pos2}, allPos)
	})

	t.Run("Receive multiple times", func(t *testing.T) {
		pos := fixtureCoords
		pos.DeviceID = "TuTu"
		err := c.Receive(ctx, coords)
		require.NoError(t, err)

		err = c.Receive(ctx, coords)
		require.NoError(t, err)
	})
}
