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

	err := c.Receive(ctx, coords)
	require.NoError(t, err)

	t.Run("Get", func(t *testing.T) {
		pos, err := c.Get(ctx, deviceID)
		require.NoError(t, err)
		assert.Equal(t, coords, pos)
	})

	t.Run("GetAll", func(t *testing.T) {
		pos := fixtureCoords
		allPos, err := c.GetAll(ctx)
		require.NoError(t, err)
		assert.Len(t, allPos, 1)
		assert.Equal(t, pos, allPos[0])

		pos.DeviceID = "jfkewkjl"
		err = c.Receive(ctx, pos)
		require.NoError(t, err)

		allPos, err = c.GetAll(ctx)
		require.NoError(t, err)
		assert.Len(t, allPos, 2)
		assert.Equal(t, pos, allPos[1])
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
