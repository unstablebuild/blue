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

	pos, err := c.Get(ctx, deviceID)
	require.NoError(t, err)
	assert.Equal(t, coords, pos)

	allPos, err := c.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, allPos, 1)
	assert.Equal(t, coords, allPos[0])

	coords.DeviceID = "jfkewkjl"
	err = c.Receive(ctx, coords)
	require.NoError(t, err)

	allPos, err = c.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, allPos, 2)
	assert.Equal(t, coords, allPos[1])
}
