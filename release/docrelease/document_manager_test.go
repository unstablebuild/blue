// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docrelease

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
)

var (
	fixtureRelease = release.Bundle{
		Package: "blue",
		Version: "1.0.0",
		Notes:   "It worked in my computer!",
		Metadata: map[string]string{
			"NeverUnderstood": "NoBueno",
			"author":          "Theranos",
		},
	}
	fixtureRelease2 = release.Bundle{
		Package: "blue",
		Version: "1.0.1",
		Metadata: map[string]string{
			"repository": "blue",
		},
	}
	fixtureSmallData = []byte("ground baking!")
	fixtureLargeData = []byte{}
)

type testProgressDelegate struct {
	progress []int64
	total    []int64
	units    []string
	fileInfo testFileInfo
}

type testFileInfo struct {
	size int64
}

func (t testFileInfo) Name() string { return "" }
func (t testFileInfo) Size() int64 {
	return t.size
}
func (t testFileInfo) Mode() os.FileMode {
	return 0
}
func (t testFileInfo) ModTime() time.Time {
	return time.Now()
}
func (t testFileInfo) IsDir() bool {
	return false
}
func (t testFileInfo) Sys() interface{} {
	return nil
}

func (d *testProgressDelegate) Progress(progress, total int64, units string) {
	d.progress = append(d.progress, progress)
	d.total = append(d.total, total)
	d.units = append(d.units, units)
}

func (d *testProgressDelegate) Stat() (os.FileInfo, error) {
	return d.fileInfo, nil
}

func init() {
	buffer := make([]byte, maxDocSizeBytes)
	for i := 0; i < 3; i++ {
		fixtureLargeData = append(fixtureLargeData, buffer...)
	}
}

func newTestingDocumentManager() (m release.Manager, svc document.Service) {
	svc = document.NewInMemoryService()
	m = NewManager(svc)
	return
}

func TestDocumentManager(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a new package", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		err := m.Create(ctx, release.Package{
			Name:   "blue",
			Notes:  "blabla",
			Latest: "", // allowed to be empty
		})
		require.NoError(t, err)
		pack, err := m.GetPackage(ctx, "blue")
		require.NoError(t, err)

		assert.Equal(t, release.Package{
			Name:     "blue",
			Notes:    "blabla",
			Metadata: map[string]string{},
		}, pack)
	})

	t.Run("deletes a package", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		err := m.Create(ctx, release.Package{
			Name:  "hopper",
			Notes: "blabla",
		})
		require.NoError(t, err)
		require.NoError(t, m.DeletePackage(ctx, "hopper"))
		pkgIter, err := m.ListPackages(ctx, nil)
		require.NoError(t, err)
		packages, err := iterator.ToSlice(context.Background(), pkgIter)
		require.NoError(t, err)

		require.Len(t, packages, 0)
	})

	t.Run("uploads a new release bundle and uploads data", func(t *testing.T) {
		m, svc := newTestingDocumentManager()
		err := m.Create(ctx, release.Package{Name: fixtureRelease.Package})
		require.NoError(t, err)
		err = m.Upload(ctx, fixtureRelease,
			release.NopProgressReader(bytes.NewBuffer(fixtureSmallData)))
		require.NoError(t, err)
		// add another release to make sure we download only from one
		err = m.Upload(ctx, fixtureRelease2,
			release.NopProgressReader(bytes.NewBuffer([]byte("stuff"))))
		require.NoError(t, err)

		it, err := svc.List(ctx, nil)
		require.NoError(t, err)

		for it.HasNext() {
			var doc interface{}
			_ = it.NextTo(&doc)
		}

		var b bytes.Buffer
		manifest, err := m.Get(
			ctx, fixtureRelease.Package,
			fixtureRelease.Version, release.NopProgressWriter(&b))
		require.NoError(t, err)
		assert.Equal(t, fixtureRelease, manifest)
		assert.Equal(t, fixtureSmallData, b.Bytes())
	})

	t.Run("uploads fails if package has not been created yet", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		bundle := fixtureRelease
		bundle.Version = "0.1.2"
		err := m.Upload(ctx, bundle,
			release.NopProgressReader(bytes.NewBuffer([]byte(""))))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("uploads a new release bundle and updates latest package version", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		bundle := fixtureRelease
		bundle.Version = "0.1.2"
		err := m.Create(ctx,
			release.Package{Name: bundle.Package, Notes: "blabla",
				Metadata: map[string]string{"1": "1"}})
		require.NoError(t, err)
		err = m.Upload(ctx, bundle,
			release.NopProgressReader(bytes.NewBuffer([]byte(""))))
		require.NoError(t, err)

		pkgIter, err := m.ListPackages(ctx, nil)
		require.NoError(t, err)
		packages, err := iterator.ToSlice(context.Background(), pkgIter)
		require.NoError(t, err)
		require.Len(t, packages, 1)
		assert.Equal(t, release.Package{
			Name:     fixtureRelease.Package,
			Latest:   release.Version("0.1.2"),
			Notes:    "blabla",
			Metadata: map[string]string{"1": "1"},
		}, packages[0])

		pkg, err := m.GetPackage(ctx, bundle.Package)
		require.NoError(t, err)
		assert.Equal(t, release.Package{
			Name:     fixtureRelease.Package,
			Latest:   release.Version("0.1.2"),
			Notes:    "blabla",
			Metadata: map[string]string{"1": "1"},
		}, pkg)
	})

	t.Run("creates a new manifest and uploads chunkified release data with progress", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		data := bytes.NewBuffer(fixtureLargeData)
		testFinfo := testFileInfo{size: int64(data.Len())}
		createProgress := testProgressDelegate{fileInfo: testFinfo}
		read := release.NewRelayProgressReader(data, &createProgress)
		err := m.Create(ctx, release.Package{Name: fixtureRelease.Package})
		require.NoError(t, err)
		err = m.Upload(ctx, fixtureRelease, read)
		assert.NoError(t, err)

		var b bytes.Buffer
		getProgress := testProgressDelegate{}
		write := release.NewRelayProgressWriter(&b, &getProgress)
		manifest, err := m.Get(ctx, fixtureRelease.Package,
			fixtureRelease.Version, write)
		require.NoError(t, err)
		assert.Equal(t, fixtureRelease, manifest)
		assert.Equal(t, fixtureLargeData, b.Bytes())
		expectedCreateProgress := testProgressDelegate{
			progress: []int64{0, 1048423, 2096846, 3145269},
			total:    []int64{3145269, 3145269, 3145269, 3145269},
			units:    []string{"bytes", "bytes", "bytes", "bytes"},
		}
		expectedGetProgress := testProgressDelegate{
			progress: []int64{0, 1, 2, 3},
			total:    []int64{3, 3, 3, 3},
			units:    []string{"chunks", "chunks", "chunks", "chunks"},
		}
		createProgress.fileInfo.size = 0 // not what we are testing
		assert.Equal(t, expectedCreateProgress, createProgress)
		assert.Equal(t, expectedGetProgress, getProgress)
	})

	t.Run("list filters out by metadata", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		err := m.Create(ctx, release.Package{Name: fixtureRelease.Package})
		require.NoError(t, err)
		err = m.Upload(ctx, fixtureRelease,
			release.NopProgressReader(bytes.NewBuffer(fixtureSmallData)))
		require.NoError(t, err)
		err = m.Upload(ctx, fixtureRelease2,
			release.NopProgressReader(bytes.NewBuffer(fixtureLargeData)))
		require.NoError(t, err)

		filters := map[string]string{"Metadata.repository": "blue"}
		it, err := m.List(ctx, fixtureRelease.Package, filters)
		require.NoError(t, err)
		items, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)

		require.Len(t, items, 1)
		assert.Equal(t, fixtureRelease2.Version, items[0].Version)
	})

	t.Run("get fails if data has been altered since point of manifest creation", func(t *testing.T) {
		m, svc := newTestingDocumentManager()
		err := m.Create(ctx, release.Package{Name: fixtureRelease.Package})
		require.NoError(t, err)
		err = m.Upload(ctx, fixtureRelease,
			release.NopProgressReader(bytes.NewBuffer(fixtureSmallData)))
		assert.NoError(t, err)

		tamperedData := releaseData{
			Type: documentTypeData,
			Data: []byte("\x00"),
		}
		err = svc.Set(ctx, makeChunkID(fixtureRelease.Package,
			fixtureRelease.Version, 0), tamperedData)
		require.NoError(t, err)

		var b bytes.Buffer
		manifest, err := m.Get(ctx, fixtureRelease.Package,
			fixtureRelease.Version, release.NopProgressWriter(&b))
		require.Equal(t, ErrDataIntegrity, err)
		require.Zero(t, manifest)
	})

	t.Run("update metadata preserves notes and latest", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		bundle := fixtureRelease
		bundle.Version = "2.3.4"
		err := m.Create(ctx, release.Package{
			Name:     bundle.Package,
			Notes:    "important notes",
			Metadata: map[string]string{"existing": "keep"},
		})
		require.NoError(t, err)
		err = m.Upload(ctx, bundle,
			release.NopProgressReader(bytes.NewBuffer([]byte(""))))
		require.NoError(t, err)

		err = m.UpdatePackageMetadata(ctx, bundle.Package,
			map[string]string{"language": "true"})
		require.NoError(t, err)

		pkg, err := m.GetPackage(ctx, bundle.Package)
		require.NoError(t, err)
		assert.Equal(t, release.Package{
			Name:   bundle.Package,
			Latest: release.Version("2.3.4"),
			Notes:  "important notes",
			Metadata: map[string]string{
				"existing": "keep",
				"language": "true",
			},
		}, pkg)
	})

	t.Run("update metadata fails for a non-existent package", func(t *testing.T) {
		m, _ := newTestingDocumentManager()
		err := m.UpdatePackageMetadata(ctx, "ghost",
			map[string]string{"language": "true"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})
}

func BenchmarkDocumentManagerUpload(b *testing.B) {
	m, _ := newTestingDocumentManager()
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		_ = m.Upload(ctx, fixtureRelease,
			release.NopProgressReader(bytes.NewBuffer(fixtureLargeData)))
	}
}

func BenchmarkDocumentManagerUploadDelete(b *testing.B) {
	m, _ := newTestingDocumentManager()
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		_ = m.Upload(ctx, fixtureRelease,
			release.NopProgressReader(bytes.NewBuffer(fixtureLargeData)))
		_ = m.Delete(ctx, fixtureRelease.Package, fixtureRelease.Version)
	}
}

func BenchmarkDocumentManagerGet(b *testing.B) {
	m, _ := newTestingDocumentManager()
	ctx := context.Background()
	_ = m.Upload(ctx, fixtureRelease,
		release.NopProgressReader(bytes.NewBuffer(fixtureLargeData)))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = m.Get(ctx, fixtureRelease.Package, fixtureRelease.Version,
			release.NopProgressWriter(io.Discard))
	}
}
