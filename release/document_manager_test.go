package release

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fixtureRelease = Manifest{
		ID:    "1.0.0",
		Notes: "It worked in my computer!",
		Metadata: map[string]string{
			"NeverUnderstood": "NoBueno",
			"author":          "Theranos",
		},
	}
	fixtureRelease2 = Manifest{
		ID: "1.0.1",
		Metadata: map[string]string{
			"repository": "blue",
		},
	}
	fixtureSmallData = []byte("ground baking!")
	fixtureLargeData = []byte{}
)

type testProgressDelegate struct {
	progress []int
	total    []int
	units    []string
}

func (d *testProgressDelegate) Progress(progress, total int, units string) {
	d.progress = append(d.progress, progress)
	d.total = append(d.total, total)
	d.units = append(d.units, units)
}

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
		err := m.Create(ctx, fixtureRelease,
			NopProgressReader(bytes.NewBuffer(fixtureSmallData)))
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
		manifest, err := m.Get(ctx, fixtureRelease.ID, NopProgressWriter(&b))
		require.NoError(t, err)
		assert.Equal(t, fixtureRelease, manifest)
		assert.Equal(t, fixtureSmallData, b.Bytes())
	})

	t.Run("creates a new manifest and uploads chunkified release data with progress", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		createProgress := testProgressDelegate{}
		read := progressDelegate{
			readDelegate:     bytes.NewBuffer(fixtureLargeData),
			progressDelegate: &createProgress,
		}
		err := m.Create(ctx, fixtureRelease, read)
		assert.NoError(t, err)

		var b bytes.Buffer
		getProgress := testProgressDelegate{}
		write := progressDelegate{
			writeDelegate:    &b,
			progressDelegate: &getProgress,
		}
		manifest, err := m.Get(ctx, fixtureRelease.ID, write)
		require.NoError(t, err)
		assert.Equal(t, fixtureRelease, manifest)
		assert.Equal(t, fixtureLargeData, b.Bytes())
		expectedCreateProgress := testProgressDelegate{
			progress: []int{0, 1048423, 2096846, 3145269},
			total:    []int{0, 0, 0, 0},
			units:    []string{"bytes", "bytes", "bytes", "bytes"},
		}
		expectedGetProgress := testProgressDelegate{
			progress: []int{0, 1, 2, 3},
			total:    []int{3, 3, 3, 3},
			units:    []string{"chunks", "chunks", "chunks", "chunks"},
		}
		assert.Equal(t, expectedCreateProgress, createProgress)
		assert.Equal(t, expectedGetProgress, getProgress)
	})

	t.Run("list filters out by metadata", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		err := m.Create(ctx, fixtureRelease,
			NopProgressReader(bytes.NewBuffer(fixtureSmallData)))
		require.NoError(t, err)
		err = m.Create(ctx, fixtureRelease2,
			NopProgressReader(bytes.NewBuffer(fixtureLargeData)))
		require.NoError(t, err)

		filters := map[string]string{"repository": "blue"}
		items, err := m.List(ctx, filters)
		require.NoError(t, err)

		assert.Len(t, items, 1)
		assert.Equal(t, fixtureRelease2.ID, items[0].ID)
	})

	t.Run("get fails if data has been altered since point of manifest creation", func(t *testing.T) {
		m, svc := newTestingDocumentManager()
		err := m.Create(ctx, fixtureRelease,
			NopProgressReader(bytes.NewBuffer(fixtureSmallData)))
		assert.NoError(t, err)

		tamperedData := releaseData{
			Type: documentTypeData,
			Data: []byte("\x00"),
		}
		err = svc.Set(ctx, makeChunkID(fixtureRelease.ID, 0), tamperedData)
		require.NoError(t, err)

		var b bytes.Buffer
		manifest, err := m.Get(ctx, fixtureRelease.ID, NopProgressWriter(&b))
		require.Equal(t, ErrDataIntegrity, err)
		require.Zero(t, manifest)
	})
}

func BenchmarkDocumentManagerCreate(b *testing.B) {
	m, _ := newTestingDocumentManager()
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		_ = m.Create(ctx, fixtureRelease,
			NopProgressReader(bytes.NewBuffer(fixtureLargeData)))
	}
}

func BenchmarkDocumentManagerCreateDelete(b *testing.B) {
	m, _ := newTestingDocumentManager()
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		_ = m.Create(ctx, fixtureRelease,
			NopProgressReader(bytes.NewBuffer(fixtureLargeData)))
		_ = m.Delete(ctx, fixtureRelease.ID)
	}
}

func BenchmarkDocumentManagerGet(b *testing.B) {
	m, _ := newTestingDocumentManager()
	ctx := context.Background()
	_ = m.Create(ctx, fixtureRelease,
		NopProgressReader(bytes.NewBuffer(fixtureLargeData)))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = m.Get(ctx, fixtureRelease.ID,
			NopProgressWriter(ioutil.Discard))
	}
}
