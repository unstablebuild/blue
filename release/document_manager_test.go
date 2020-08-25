package release

import (
	"bytes"
	"context"
	"testing"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fixtureRelease = Manifest{
		ID:     "1.0.0",
		Author: "Theranos",
		Notes:  "It worked in my computer!",
	}
	fixtureSmallData = []byte("ground baking!")
	fixtureLargeData = []byte{}
)

func init() {
	buffer := make([]byte, maxDocSizeBytes)
	for i := 0; i < 3; i++ {
		fixtureLargeData = append(fixtureLargeData, buffer...)
	}
}

func newTestingDocumentManager() (m Manager, svc document.Service) {
	svc = document.NewInMemoryCache()
	m = NewDocumentManager(svc)
	return
}

func TestDocumentManager(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a new manifest and uploads release data", func(t *testing.T) {
		m, svc := newTestingDocumentManager()
		err := m.Create(ctx, fixtureRelease, bytes.NewBuffer(fixtureSmallData))
		assert.NoError(t, err)

		it, err := svc.List(ctx, nil)
		require.NoError(t, err)

		i := 0
		for ; it.HasNext(); i++ {
			var doc interface{}
			_ = it.NextTo(&doc)
		}
		assert.Equal(t, 2, i)

		var b bytes.Buffer
		manifest, err := m.Get(ctx, fixtureRelease.ID, &b)
		require.NoError(t, err)
		assert.Equal(t, fixtureRelease, manifest)
		assert.Equal(t, fixtureSmallData, b.Bytes())
	})

	t.Run("creates a new manifest and uploads chunkified release data", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		err := m.Create(ctx, fixtureRelease, bytes.NewBuffer(fixtureLargeData))
		assert.NoError(t, err)

		var b bytes.Buffer
		manifest, err := m.Get(ctx, fixtureRelease.ID, &b)
		require.NoError(t, err)
		assert.Equal(t, fixtureRelease, manifest)
		assert.Equal(t, fixtureLargeData, b.Bytes())
	})
}
