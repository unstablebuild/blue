package gps

import (
	"context"
	"testing"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheReceiver(t *testing.T) {
	b := document.NewInMemoryCache()
	c := NewCache(b)

	ctx := context.Background()

	err := c.Receive(ctx, fixtureCoords)
	require.NoError(t, err)

	pos, err := c.Get(ctx, deviceID)
	require.NoError(t, err)
	assert.Equal(t, fixtureCoords, pos)
}
