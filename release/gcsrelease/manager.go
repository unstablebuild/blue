// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package gcsrelease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
	"google.golang.org/api/googleapi"
)

const sha256MetadataKey = "sha256"

const defaultSignedURLExpiry = 15 * time.Minute

// ErrReleaseExists is returned by Upload when the version is already
// published. Published versions are immutable; delete the release first
// to re-publish it.
var ErrReleaseExists = errors.New("release version already exists; delete it first to re-publish")

// Signer can produce short-lived signed URLs for GCS objects.
type Signer interface {
	SignedDownloadURL(ctx context.Context, pkg string, version release.Version) (string, error)
}

// Option configures the Manager.
type Option func(*Manager)

// WithPrefix sets a key prefix for GCS objects.
func WithPrefix(prefix string) Option {
	return func(m *Manager) { m.prefix = prefix }
}

// WithSignedURLOptions sets options applied when generating signed URLs.
func WithSignedURLOptions(opts *storage.SignedURLOptions) Option {
	return func(m *Manager) { m.signOpts = opts }
}

// Manager decorates a release.Manager, storing binary data in GCS
// while delegating metadata operations to the inner manager.
type Manager struct {
	inner    release.Manager
	bucket   Bucket
	prefix   string
	signOpts *storage.SignedURLOptions
}

// NewManager returns a Manager that stores binary data in bucket and
// delegates metadata to inner.
func NewManager(inner release.Manager, bucket Bucket, opts ...Option) *Manager {
	m := &Manager{inner: inner, bucket: bucket}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *Manager) objectName(pkg string, version release.Version) string {
	return path.Join(m.prefix, pkg, string(version))
}

func (m *Manager) object(pkg string, version release.Version) Object {
	return m.bucket.Object(m.objectName(pkg, version))
}

// Create delegates to the inner manager.
func (m *Manager) Create(ctx context.Context, p release.Package) error {
	return m.inner.Create(ctx, p)
}

// UpdatePackageMetadata delegates to the inner manager.
func (m *Manager) UpdatePackageMetadata(ctx context.Context, pkg string, metadata map[string]string) error {
	return m.inner.UpdatePackageMetadata(ctx, pkg, metadata)
}

// DeletePackage delegates to the inner manager.
func (m *Manager) DeletePackage(ctx context.Context, pkg string) error {
	return m.inner.DeletePackage(ctx, pkg)
}

// GetPackage delegates to the inner manager.
func (m *Manager) GetPackage(ctx context.Context, pkg string) (release.Package, error) {
	return m.inner.GetPackage(ctx, pkg)
}

// ListPackages delegates to the inner manager.
func (m *Manager) ListPackages(ctx context.Context, filters map[string]string) (iterator.Iterator[release.Package], error) {
	return m.inner.ListPackages(ctx, filters)
}

// List delegates to the inner manager.
func (m *Manager) List(ctx context.Context, pkg string, filters map[string]string) (iterator.Iterator[release.Bundle], error) {
	return m.inner.List(ctx, pkg, filters)
}

// Upload writes binary data to GCS and metadata to the inner manager.
// Published versions are immutable: re-uploading an existing version
// fails with ErrReleaseExists before any data is transferred, and the
// create-only GCS writer guarantees that even a concurrent publish can
// never replace an already-published artifact.
func (m *Manager) Upload(ctx context.Context, bundle release.Bundle, r release.ProgressReader) error {
	_, err := m.inner.Get(ctx, bundle.Package, bundle.Version,
		release.NopProgressWriter(io.Discard))
	if err == nil {
		return fmt.Errorf("%s@%s: %w", bundle.Package, bundle.Version, ErrReleaseExists)
	}

	obj := m.object(bundle.Package, bundle.Version)
	w := obj.NewCreateWriter(ctx)
	hasher := sha256.New()
	tee := io.TeeReader(r, hasher)

	var totalSize int64
	if stater, ok := r.(interface{ Stat() (os.FileInfo, error) }); ok {
		fi, err := stater.Stat()
		if err == nil {
			totalSize = fi.Size()
		}
	}

	r.Progress(0, totalSize, "bytes")
	var totalRead int64
	buf := make([]byte, 32*1024)
	for {
		n, rerr := tee.Read(buf)
		if n > 0 {
			_, werr := w.Write(buf[:n])
			if werr != nil {
				m.abortUpload(w, bundle)
				return fmt.Errorf("gcs write: %w", translateExistsErr(werr, bundle))
			}
			totalRead += int64(n)
			r.Progress(totalRead, totalSize, "bytes")
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			m.abortUpload(w, bundle)
			return fmt.Errorf("read release data: %w", rerr)
		}
	}

	// A cut pipe is indistinguishable from a clean EOF, and the checksum
	// is computed from the streamed bytes, so a short read would publish
	// a truncated artifact that passes integrity checks.
	if totalSize > 0 && totalRead != totalSize {
		m.abortUpload(w, bundle)
		return fmt.Errorf("release data truncated: read %d of %d bytes",
			totalRead, totalSize)
	}

	if err := w.Close(); err != nil {
		// Writers never overwrite, so a failed close committed nothing
		// and any object under this name predates the call.
		return fmt.Errorf("gcs close: %w", translateExistsErr(err, bundle))
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	if bundle.Metadata == nil {
		bundle.Metadata = make(map[string]string)
	}
	bundle.Metadata[sha256MetadataKey] = checksum

	if err := m.inner.Upload(ctx, bundle, release.NopProgressReader(bytes.NewReader(nil))); err != nil {
		m.forceDeleteObject(bundle.Package, bundle.Version)
		return err
	}
	return nil
}

// translateExistsErr maps the create-only writer's precondition failure
// (the artifact object already exists, e.g. orphaned by an earlier
// partial publish) onto ErrReleaseExists so callers see one sentinel for
// every "already published" shape.
func translateExistsErr(err error, bundle release.Bundle) error {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) && gerr.Code == http.StatusPreconditionFailed {
		return fmt.Errorf("%s@%s: %w", bundle.Package, bundle.Version, ErrReleaseExists)
	}
	return err
}

// abortUpload finalises an interrupted write and removes the object only
// when that finalise succeeded, which is the sole case in which this call
// created it. Closing a partial write otherwise commits a truncated
// object, so it cannot simply be left behind.
func (m *Manager) abortUpload(w io.WriteCloser, bundle release.Bundle) {
	if err := w.Close(); err != nil {
		return
	}
	m.forceDeleteObject(bundle.Package, bundle.Version)
}

// Get fetches bundle metadata from the inner manager and optionally
// streams binary data from GCS, verifying the SHA256 checksum.
func (m *Manager) Get(ctx context.Context, pkg string, ver release.Version, out release.ProgressWriter) (release.Bundle, error) {
	if ver == release.Latest {
		p, err := m.inner.GetPackage(ctx, pkg)
		if err != nil {
			return release.Bundle{}, fmt.Errorf("resolve latest: %w", err)
		}
		ver = p.Latest
	}

	bundle, err := m.inner.Get(ctx, pkg, ver, release.NopProgressWriter(io.Discard))
	if err != nil {
		return release.Bundle{}, err
	}

	if del, ok := out.(release.IsDiscard); ok && del.IsDiscard() {
		return bundle, nil
	}

	obj := m.object(pkg, ver)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return release.Bundle{}, fmt.Errorf("gcs read: %w", err)
	}
	defer func() { _ = reader.Close() }()

	hasher := sha256.New()
	tee := io.TeeReader(reader, hasher)

	totalSize := reader.Size()
	out.Progress(0, totalSize, "bytes")
	var totalRead int64
	buf := make([]byte, 32*1024)
	for {
		n, rerr := tee.Read(buf)
		if n > 0 {
			_, werr := out.Write(buf[:n])
			if werr != nil {
				return release.Bundle{}, fmt.Errorf("write release data: %w", werr)
			}
			totalRead += int64(n)
			out.Progress(totalRead, totalSize, "bytes")
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return release.Bundle{}, fmt.Errorf("gcs read data: %w", rerr)
		}
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	if expected, ok := bundle.Metadata[sha256MetadataKey]; ok && checksum != expected {
		return release.Bundle{}, fmt.Errorf("data integrity check failed: expected sha256 %s, got %s", expected, checksum)
	}

	return bundle, nil
}

// Delete removes metadata via the inner manager and then deletes the GCS
// object. A missing metadata document does not stop the object delete: a
// publish that dies between the blob commit and the metadata write
// orphans an object whose presence blocks re-publishing the version, and
// Delete is the only cleanup path.
func (m *Manager) Delete(ctx context.Context, pkg string, ver release.Version) error {
	innerErr := m.inner.Delete(ctx, pkg, ver)
	if innerErr != nil && !isNotFound(innerErr) {
		return innerErr
	}
	err := m.object(pkg, ver).Delete(ctx)
	if err != nil && !isObjectNotExist(err) {
		return fmt.Errorf("gcs delete: %w", err)
	}
	if err != nil {
		// Nothing was deleted on either side: surface the metadata miss.
		return innerErr
	}
	return nil
}

// SignedDownloadURL returns a short-lived signed URL for the given package version.
func (m *Manager) SignedDownloadURL(ctx context.Context, pkg string, version release.Version) (string, error) {
	opts := storage.SignedURLOptions{
		Method: "GET",
	}
	if m.signOpts != nil {
		opts = *m.signOpts
	}
	if opts.Method == "" {
		opts.Method = "GET"
	}
	now := time.Now()
	if !opts.Expires.After(now) {
		opts.Expires = now.Add(defaultSignedURLExpiry)
	}
	return m.bucket.SignedURL(m.objectName(pkg, version), &opts)
}

func (m *Manager) forceDeleteObject(pkg string, ver release.Version) {
	err := m.object(pkg, ver).Delete(context.Background())
	if err != nil && !isObjectNotExist(err) {
		logrus.WithFields(logrus.Fields{
			"package": pkg,
			"version": string(ver),
		}).Errorf("failed to clean up GCS object: %v", err)
	}
}

// isObjectNotExist reports whether err indicates the GCS object was already
// absent. The storage client returns storage.ErrObjectNotExist in most cases,
// but a delete of a missing object over the JSON API surfaces a
// *googleapi.Error with a 404 status that is not the sentinel, so we treat both
// as "not found" to keep deletes idempotent.
func isObjectNotExist(err error) bool {
	if errors.Is(err, storage.ErrObjectNotExist) {
		return true
	}
	var gerr *googleapi.Error
	return errors.As(err, &gerr) && gerr.Code == http.StatusNotFound
}
