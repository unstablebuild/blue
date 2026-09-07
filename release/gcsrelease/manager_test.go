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

package gcsrelease

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/release"
	"google.golang.org/api/googleapi"
)

// TestManagerUploadFailsFastWhenVersionPublished covers the re-publish
// path that wiped published rune-agent artifacts: the version is already
// in the metadata store, so Upload must fail with ErrReleaseExists before
// transferring a single byte or touching the published artifact.
func TestManagerUploadFailsFastWhenVersionPublished(t *testing.T) {
	t.Parallel()
	b := newUploadBucket()
	b.object.exists = true
	inner := &fakeInner{exists: true}
	m := NewManager(inner, b)

	err := m.Upload(context.Background(), release.Bundle{
		Package: "rune-agent", Version: release.Version("v1.1.2"),
	}, release.NopProgressReader(newStringReader("payload")))

	require.ErrorIs(t, err, ErrReleaseExists)
	require.Contains(t, err.Error(), "rune-agent@v1.1.2")
	require.Zero(t, b.object.written, "no data must be transferred")
	require.False(t, b.object.deleted, "must not delete a pre-existing release artifact")
	require.True(t, b.object.exists, "pre-existing release artifact must survive")
}

// TestManagerUploadKeepsExistingObjectOnPublishRace covers a publish that
// slips past the fail-fast check: the create-only writer refuses to
// replace the object and the existing artifact must survive untouched.
func TestManagerUploadKeepsExistingObjectOnPublishRace(t *testing.T) {
	t.Parallel()
	b := newUploadBucket()
	b.object.exists = true
	m := NewManager(&fakeInner{}, b)

	err := m.Upload(context.Background(), release.Bundle{
		Package: "rune-agent", Version: release.Version("v1.1.2"),
	}, release.NopProgressReader(newStringReader("payload")))

	require.ErrorIs(t, err, ErrReleaseExists)
	require.False(t, b.object.deleted, "must not delete a pre-existing release artifact")
	require.True(t, b.object.exists, "pre-existing release artifact must survive")
}

// TestManagerUploadDeletesOwnObjectWhenMetadataRejectsVersion pins the
// rollback scope when the duplicate is only detected at the metadata
// write: the rollback may remove the object this call created, never a
// survivor from an earlier publish.
func TestManagerUploadDeletesOwnObjectWhenMetadataRejectsVersion(t *testing.T) {
	t.Parallel()
	b := newUploadBucket()
	inner := &fakeInner{uploadErr: errors.New("document already exists")}
	m := NewManager(inner, b)

	err := m.Upload(context.Background(), release.Bundle{
		Package: "rune-agent", Version: release.Version("v1.1.2"),
	}, release.NopProgressReader(newStringReader("payload")))

	require.Error(t, err)
	require.True(t, b.object.deleted, "own orphaned artifact must be rolled back")
}

func TestManagerUploadDeletesObjectItCreatedWhenMetadataFails(t *testing.T) {
	t.Parallel()
	b := newUploadBucket()
	inner := &fakeInner{uploadErr: errors.New("boom")}
	m := NewManager(inner, b)

	err := m.Upload(context.Background(), release.Bundle{
		Package: "rune-agent", Version: release.Version("v1.1.3"),
	}, release.NopProgressReader(newStringReader("payload")))

	require.Error(t, err)
	require.True(t, b.object.deleted, "orphaned artifact must be rolled back")
}

func TestManagerUploadDoesNotDeleteWhenWriteNeverCommits(t *testing.T) {
	t.Parallel()
	b := newUploadBucket()
	b.object.exists = true
	b.object.writeErr = errors.New("connection reset")
	m := NewManager(&fakeInner{}, b)

	err := m.Upload(context.Background(), release.Bundle{
		Package: "rune-agent", Version: release.Version("v1.1.2"),
	}, release.NopProgressReader(newStringReader("payload")))

	require.Error(t, err)
	require.False(t, b.object.deleted, "an uncommitted write owns no object to delete")
	require.True(t, b.object.exists)
}

func TestManagerUploadDeletesTruncatedObjectItCommitted(t *testing.T) {
	t.Parallel()
	b := newUploadBucket()
	b.object.writeErr = errors.New("connection reset")
	m := NewManager(&fakeInner{}, b)

	err := m.Upload(context.Background(), release.Bundle{
		Package: "rune-agent", Version: release.Version("v1.1.3"),
	}, release.NopProgressReader(newStringReader("payload")))

	require.Error(t, err)
	require.True(t, b.object.deleted, "truncated artifact must be rolled back")
}

func newUploadBucket() *uploadBucket {
	return &uploadBucket{object: &uploadObject{}}
}

type uploadBucket struct {
	object *uploadObject
}

func (b *uploadBucket) Object(string) Object { return b.object }
func (b *uploadBucket) SignedURL(string, *storage.SignedURLOptions) (string, error) {
	return "", nil
}

// uploadObject models the GCS DoesNotExist precondition: a write is only
// committed at Close, and only when no object exists yet.
type uploadObject struct {
	exists   bool
	deleted  bool
	written  int
	writeErr error
}

func (o *uploadObject) NewCreateWriter(context.Context) io.WriteCloser {
	return &uploadWriter{obj: o}
}

func (o *uploadObject) NewReader(context.Context) (ObjectReader, error) {
	return nil, storage.ErrObjectNotExist
}

func (o *uploadObject) Delete(context.Context) error {
	if !o.exists {
		return storage.ErrObjectNotExist
	}
	o.exists = false
	o.deleted = true
	return nil
}

type uploadWriter struct {
	obj *uploadObject
}

func (w *uploadWriter) Write(p []byte) (int, error) {
	if w.obj.writeErr != nil {
		return 0, w.obj.writeErr
	}
	w.obj.written += len(p)
	return len(p), nil
}

func (w *uploadWriter) Close() error {
	if w.obj.exists {
		return &googleapi.Error{Code: 412, Message: "conditionNotMet"}
	}
	w.obj.exists = true
	return nil
}

func newStringReader(s string) io.Reader { return strings.NewReader(s) }

func TestManagerUploadSucceedsWhenStreamMatchesStat(t *testing.T) {
	t.Parallel()
	b := newUploadBucket()
	inner := &fakeInner{}
	m := NewManager(inner, b)

	err := m.Upload(context.Background(), release.Bundle{
		Package: "rune-agent", Version: release.Version("v1.1.3"),
	}, &statReader{Reader: strings.NewReader("payload"), size: int64(len("payload"))})

	require.NoError(t, err)
	require.True(t, inner.uploaded, "metadata must be written")
	require.True(t, b.object.exists, "artifact must be committed")
	require.Equal(t, len("payload"), b.object.written)
}

// TestManagerUploadRejectsTruncatedStream pins the EOF-truncation hole: a
// cut pipe surfaces as a clean EOF, and without a size check the upload
// would publish a truncated artifact whose checksum matches the
// truncated bytes — with package Latest then pointing at it.
func TestManagerUploadRejectsTruncatedStream(t *testing.T) {
	t.Parallel()
	b := newUploadBucket()
	inner := &fakeInner{}
	m := NewManager(inner, b)

	err := m.Upload(context.Background(), release.Bundle{
		Package: "rune-agent", Version: release.Version("v1.1.3"),
	}, &statReader{Reader: strings.NewReader("payload"), size: 9999})

	require.Error(t, err)
	require.Contains(t, err.Error(), "truncated")
	require.False(t, inner.uploaded, "no metadata may be written for a truncated stream")
	require.False(t, b.object.exists, "truncated artifact must not survive")
}

// statReader is a release.ProgressReader whose Stat reports size
// regardless of how many bytes the stream actually yields.
type statReader struct {
	io.Reader
	size int64
}

func (r *statReader) Progress(int64, int64, string) {}

func (r *statReader) Stat() (os.FileInfo, error) { return fakeFileInfo{size: r.size}, nil }

type fakeFileInfo struct{ size int64 }

func (fakeFileInfo) Name() string           { return "release.tar.gz" }
func (f fakeFileInfo) Size() int64          { return f.size }
func (fakeFileInfo) Mode() os.FileMode      { return 0 }
func (fakeFileInfo) ModTime() (t time.Time) { return }
func (fakeFileInfo) IsDir() bool            { return false }
func (fakeFileInfo) Sys() any               { return nil }

func TestManagerSignedDownloadURLDefaultsExpiry(t *testing.T) {
	t.Parallel()
	b := &capturingBucket{}
	m := NewManager(nil, b)

	_, err := m.SignedDownloadURL(context.Background(), "pkg", release.Version("1"))
	require.NoError(t, err)
	require.NotNil(t, b.lastOpts)
	require.Equal(t, "GET", b.lastOpts.Method)
	require.False(t, b.lastOpts.Expires.IsZero())
	require.True(t, b.lastOpts.Expires.After(time.Now()))
	require.Equal(t, "pkg/1", b.lastObject)
}

func TestManagerSignedDownloadURLPreservesFutureExpiry(t *testing.T) {
	t.Parallel()
	want := time.Now().Add(42 * time.Minute).Round(time.Second)
	b := &capturingBucket{}
	m := NewManager(nil, b, WithSignedURLOptions(&storage.SignedURLOptions{
		Method:  "HEAD",
		Expires: want,
	}))

	_, err := m.SignedDownloadURL(context.Background(), "pkg", release.Version("1"))
	require.NoError(t, err)
	require.Equal(t, "HEAD", b.lastOpts.Method)
	require.Equal(t, want, b.lastOpts.Expires)
}

type capturingBucket struct {
	lastObject string
	lastOpts   *storage.SignedURLOptions
}

func (b *capturingBucket) Object(name string) Object {
	return capturingObject{}
}

func (b *capturingBucket) SignedURL(object string, opts *storage.SignedURLOptions) (string, error) {
	b.lastObject = object
	clone := *opts
	b.lastOpts = &clone
	return "https://example.invalid", nil
}

type capturingObject struct{}

func (capturingObject) NewCreateWriter(context.Context) io.WriteCloser  { panic("unused") }
func (capturingObject) NewReader(context.Context) (ObjectReader, error) { panic("unused") }
func (capturingObject) Delete(context.Context) error                    { panic("unused") }

func TestManagerDeleteToleratesMissingObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		delErr error
	}{
		{name: "sentinel", delErr: storage.ErrObjectNotExist},
		{name: "googleapi 404", delErr: &googleapi.Error{Code: 404, Message: "No such object"}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inner := &fakeInner{}
			b := &deleteBucket{delErr: tc.delErr}
			m := NewManager(inner, b)

			err := m.Delete(context.Background(), "pkg", release.Version("1"))
			require.NoError(t, err)
			require.True(t, inner.deleted, "inner metadata delete should run")
		})
	}
}

func TestManagerDeletePropagatesRealError(t *testing.T) {
	t.Parallel()
	inner := &fakeInner{}
	b := &deleteBucket{delErr: &googleapi.Error{Code: 500, Message: "boom"}}
	m := NewManager(inner, b)

	err := m.Delete(context.Background(), "pkg", release.Version("1"))
	require.Error(t, err)
}

// TestManagerDeleteCleansOrphanObject covers the crash-window cleanup: a
// publish that died between the blob commit and the metadata write
// leaves an object with no document, and its presence blocks
// re-publishing the version. Delete is the only cleanup path, so a
// missing document must not stop the object delete.
func TestManagerDeleteCleansOrphanObject(t *testing.T) {
	t.Parallel()
	inner := &fakeInner{deleteErr: errors.New("document not found")}
	b := &deleteBucket{}
	m := NewManager(inner, b)

	err := m.Delete(context.Background(), "rune-agent", release.Version("v1.1.3"))
	require.NoError(t, err)
}

func TestManagerDeleteFailsWhenNothingExists(t *testing.T) {
	t.Parallel()
	inner := &fakeInner{deleteErr: errors.New("document not found")}
	b := &deleteBucket{delErr: storage.ErrObjectNotExist}
	m := NewManager(inner, b)

	err := m.Delete(context.Background(), "rune-agent", release.Version("v1.1.3"))
	require.EqualError(t, err, "document not found")
}

// fakeInner is a minimal release.Manager for Upload and Delete flows.
type fakeInner struct {
	release.Manager
	deleted   bool
	exists    bool
	uploaded  bool
	uploadErr error
	deleteErr error
}

func (f *fakeInner) Get(
	context.Context, string, release.Version, release.ProgressWriter,
) (release.Bundle, error) {
	if f.exists {
		return release.Bundle{}, nil
	}
	return release.Bundle{}, errors.New("document not found")
}

func (f *fakeInner) Upload(context.Context, release.Bundle, release.ProgressReader) error {
	if f.uploadErr != nil {
		return f.uploadErr
	}
	f.uploaded = true
	return nil
}

func (f *fakeInner) Delete(context.Context, string, release.Version) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = true
	return nil
}

type deleteBucket struct {
	delErr error
}

func (b *deleteBucket) Object(string) Object { return deleteObject{err: b.delErr} }
func (b *deleteBucket) SignedURL(string, *storage.SignedURLOptions) (string, error) {
	return "", nil
}

type deleteObject struct{ err error }

func (deleteObject) NewCreateWriter(context.Context) io.WriteCloser  { panic("unused") }
func (deleteObject) NewReader(context.Context) (ObjectReader, error) { panic("unused") }
func (o deleteObject) Delete(context.Context) error                  { return o.err }
