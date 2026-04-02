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

package cdnrelease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
)

// mockReleaseManager implements release.Manager with function fields.
type mockReleaseManager struct {
	createFn        func(ctx context.Context, p release.Package) error
	deletePackageFn func(ctx context.Context, pkg string) error
	getPackageFn    func(ctx context.Context, pkg string) (release.Package, error)
	listPackagesFn  func(ctx context.Context, filters map[string]string) (iterator.Iterator[release.Package], error)
	uploadFn        func(ctx context.Context, b release.Bundle, r release.ProgressReader) error
	getFn           func(ctx context.Context, pkg string, ver release.Version, w release.ProgressWriter) (release.Bundle, error)
	deleteFn        func(ctx context.Context, pkg string, ver release.Version) error
	listFn          func(ctx context.Context, pkg string, filters map[string]string) (iterator.Iterator[release.Bundle], error)
}

func (m *mockReleaseManager) Create(ctx context.Context, p release.Package) error {
	return m.createFn(ctx, p)
}
func (m *mockReleaseManager) DeletePackage(ctx context.Context, pkg string) error {
	return m.deletePackageFn(ctx, pkg)
}
func (m *mockReleaseManager) GetPackage(ctx context.Context, pkg string) (release.Package, error) {
	return m.getPackageFn(ctx, pkg)
}
func (m *mockReleaseManager) ListPackages(ctx context.Context, filters map[string]string) (iterator.Iterator[release.Package], error) {
	return m.listPackagesFn(ctx, filters)
}
func (m *mockReleaseManager) Upload(ctx context.Context, b release.Bundle, r release.ProgressReader) error {
	return m.uploadFn(ctx, b, r)
}
func (m *mockReleaseManager) Get(ctx context.Context, pkg string, ver release.Version, w release.ProgressWriter) (release.Bundle, error) {
	return m.getFn(ctx, pkg, ver, w)
}
func (m *mockReleaseManager) Delete(ctx context.Context, pkg string, ver release.Version) error {
	return m.deleteFn(ctx, pkg, ver)
}
func (m *mockReleaseManager) List(ctx context.Context, pkg string, filters map[string]string) (iterator.Iterator[release.Bundle], error) {
	return m.listFn(ctx, pkg, filters)
}

type mockSigner struct {
	signFn func(ctx context.Context, pkg string, ver release.Version) (string, error)
}

func (s *mockSigner) SignedDownloadURL(ctx context.Context, pkg string, ver release.Version) (string, error) {
	return s.signFn(ctx, pkg, ver)
}

func computeSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// newTestSetup creates a gcsrelease.Handler-backed httptest.Server and
// a data server for signed URL downloads.
func newTestSetup(
	inner release.Manager,
	signer signer,
	dataHandler http.Handler,
) (cdnManager *Manager, handlerServer *httptest.Server, dataServer *httptest.Server) {
	// Import from gcsrelease is avoided by using the same interface locally.
	// Build the handler inline using the same mux pattern as gcsrelease.NewHandler.
	h := &testHandler{m: inner, signer: signer}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /packages", h.listPackages)
	mux.HandleFunc("GET /packages/{name}", h.getPackage)
	mux.HandleFunc("GET /packages/{name}/bundles", h.listBundles)
	mux.HandleFunc("GET /packages/{name}/bundles/{version}", h.getBundle)
	mux.HandleFunc("GET /packages/{name}/bundles/{version}/download", h.downloadBundle)

	handlerServer = httptest.NewServer(mux)
	if dataHandler != nil {
		dataServer = httptest.NewServer(dataHandler)
	}
	cdnManager = NewManager(handlerServer.Client(), handlerServer.URL)
	return
}

// signer interface matches gcsrelease.Signer without importing it.
type signer interface {
	SignedDownloadURL(ctx context.Context, pkg string, version release.Version) (string, error)
}

// testHandler duplicates the gcsrelease handler logic for integration testing
// to avoid import cycles.
type testHandler struct {
	m      release.Manager
	signer signer
}

func (h *testHandler) listPackages(w http.ResponseWriter, r *http.Request) {
	filters := queryFilters(r)
	it, err := h.m.ListPackages(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	pkgs, err := iterator.ToSlice(r.Context(), it)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, pkgs)
}

func (h *testHandler) getPackage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pkg, err := h.m.GetPackage(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, pkg)
}

func (h *testHandler) listBundles(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	filters := queryFilters(r)
	it, err := h.m.List(r.Context(), name, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	bundles, err := iterator.ToSlice(r.Context(), it)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, bundles)
}

func (h *testHandler) getBundle(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := release.Version(r.PathValue("version"))
	bundle, err := h.m.Get(r.Context(), name, version, release.NopProgressWriter(io.Discard))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

func (h *testHandler) downloadBundle(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := release.Version(r.PathValue("version"))
	bundle, err := h.m.Get(r.Context(), name, version, release.NopProgressWriter(io.Discard))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	url, err := h.signer.SignedDownloadURL(r.Context(), name, version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, downloadResponse{Bundle: bundle, URL: url})
}

func queryFilters(r *http.Request) map[string]string {
	filters := make(map[string]string)
	for k, vs := range r.URL.Query() {
		if len(vs) > 0 {
			filters[k] = vs[0]
		}
	}
	if len(filters) == 0 {
		return nil
	}
	return filters
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

var errMockNotFound = errors.New("document not found")

func TestCDNManager(t *testing.T) {
	ctx := context.Background()

	t.Run("GetPackage found", func(t *testing.T) {
		pkg := release.Package{Name: "blue", Latest: "1.0.0"}
		inner := &mockReleaseManager{
			getPackageFn: func(_ context.Context, name string) (release.Package, error) {
				if name == "blue" {
					return pkg, nil
				}
				return release.Package{}, errMockNotFound
			},
		}
		m, srv, _ := newTestSetup(inner, nil, nil)
		defer srv.Close()

		got, err := m.GetPackage(ctx, "blue")
		require.NoError(t, err)
		assert.Equal(t, pkg, got)
	})

	t.Run("GetPackage not found", func(t *testing.T) {
		inner := &mockReleaseManager{
			getPackageFn: func(_ context.Context, _ string) (release.Package, error) {
				return release.Package{}, errMockNotFound
			},
		}
		m, srv, _ := newTestSetup(inner, nil, nil)
		defer srv.Close()

		_, err := m.GetPackage(ctx, "nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("ListPackages results", func(t *testing.T) {
		pkgs := []release.Package{
			{Name: "alpha", Latest: "1.0.0"},
			{Name: "beta", Latest: "2.0.0"},
		}
		inner := &mockReleaseManager{
			listPackagesFn: func(_ context.Context, _ map[string]string) (iterator.Iterator[release.Package], error) {
				return iterator.FromSlice(pkgs), nil
			},
		}
		m, srv, _ := newTestSetup(inner, nil, nil)
		defer srv.Close()

		it, err := m.ListPackages(ctx, nil)
		require.NoError(t, err)
		got, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		assert.Equal(t, pkgs, got)
	})

	t.Run("ListPackages empty", func(t *testing.T) {
		inner := &mockReleaseManager{
			listPackagesFn: func(_ context.Context, _ map[string]string) (iterator.Iterator[release.Package], error) {
				return iterator.FromSlice[release.Package](nil), nil
			},
		}
		m, srv, _ := newTestSetup(inner, nil, nil)
		defer srv.Close()

		it, err := m.ListPackages(ctx, nil)
		require.NoError(t, err)
		got, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("ListPackages with filters", func(t *testing.T) {
		var capturedFilters map[string]string
		inner := &mockReleaseManager{
			listPackagesFn: func(_ context.Context, filters map[string]string) (iterator.Iterator[release.Package], error) {
				capturedFilters = filters
				return iterator.FromSlice[release.Package](nil), nil
			},
		}
		m, srv, _ := newTestSetup(inner, nil, nil)
		defer srv.Close()

		_, err := m.ListPackages(ctx, map[string]string{"os": "linux"})
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"os": "linux"}, capturedFilters)
	})

	t.Run("List bundles", func(t *testing.T) {
		bundles := []release.Bundle{
			{Package: "blue", Version: "1.0.0"},
			{Package: "blue", Version: "1.0.1"},
		}
		inner := &mockReleaseManager{
			listFn: func(_ context.Context, _ string, _ map[string]string) (iterator.Iterator[release.Bundle], error) {
				return iterator.FromSlice(bundles), nil
			},
		}
		m, srv, _ := newTestSetup(inner, nil, nil)
		defer srv.Close()

		it, err := m.List(ctx, "blue", nil)
		require.NoError(t, err)
		got, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		assert.Equal(t, bundles, got)
	})

	t.Run("List bundles empty", func(t *testing.T) {
		inner := &mockReleaseManager{
			listFn: func(_ context.Context, _ string, _ map[string]string) (iterator.Iterator[release.Bundle], error) {
				return iterator.FromSlice[release.Bundle](nil), nil
			},
		}
		m, srv, _ := newTestSetup(inner, nil, nil)
		defer srv.Close()

		it, err := m.List(ctx, "blue", nil)
		require.NoError(t, err)
		got, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("Get with data full round-trip", func(t *testing.T) {
		data := []byte("hello release data")
		checksum := computeSHA256(data)
		bundle := release.Bundle{
			Package:  "blue",
			Version:  "1.0.0",
			Metadata: map[string]string{sha256MetadataKey: checksum},
		}

		inner := &mockReleaseManager{
			getFn: func(_ context.Context, _ string, _ release.Version, _ release.ProgressWriter) (release.Bundle, error) {
				return bundle, nil
			},
		}

		// Data server serves the binary data.
		dataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(data)
		}))
		defer dataServer.Close()

		signerMock := &mockSigner{
			signFn: func(_ context.Context, _ string, _ release.Version) (string, error) {
				return dataServer.URL + "/data", nil
			},
		}

		m, srv, _ := newTestSetup(inner, signerMock, nil)
		defer srv.Close()

		var buf bytes.Buffer
		got, err := m.Get(ctx, "blue", "1.0.0", release.NopProgressWriter(&buf))
		require.NoError(t, err)
		assert.Equal(t, bundle, got)
		assert.Equal(t, data, buf.Bytes())
	})

	t.Run("Get with Discard skips download", func(t *testing.T) {
		bundle := release.Bundle{
			Package:  "blue",
			Version:  "1.0.0",
			Metadata: map[string]string{sha256MetadataKey: "abc"},
		}

		inner := &mockReleaseManager{
			getFn: func(_ context.Context, _ string, _ release.Version, _ release.ProgressWriter) (release.Bundle, error) {
				return bundle, nil
			},
		}

		dataServerCalled := false
		dataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dataServerCalled = true
			_, _ = w.Write([]byte("should not be called"))
		}))
		defer dataServer.Close()

		signerMock := &mockSigner{
			signFn: func(_ context.Context, _ string, _ release.Version) (string, error) {
				return dataServer.URL + "/data", nil
			},
		}

		m, srv, _ := newTestSetup(inner, signerMock, nil)
		defer srv.Close()

		got, err := m.Get(ctx, "blue", "1.0.0", release.NopProgressWriter(io.Discard))
		require.NoError(t, err)
		assert.Equal(t, bundle, got)
		assert.False(t, dataServerCalled, "data server should not be called with Discard writer")
	})

	t.Run("Get data integrity failure", func(t *testing.T) {
		bundle := release.Bundle{
			Package:  "blue",
			Version:  "1.0.0",
			Metadata: map[string]string{sha256MetadataKey: "deadbeef"},
		}

		inner := &mockReleaseManager{
			getFn: func(_ context.Context, _ string, _ release.Version, _ release.ProgressWriter) (release.Bundle, error) {
				return bundle, nil
			},
		}

		dataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("tampered data"))
		}))
		defer dataServer.Close()

		signerMock := &mockSigner{
			signFn: func(_ context.Context, _ string, _ release.Version) (string, error) {
				return dataServer.URL + "/data", nil
			},
		}

		m, srv, _ := newTestSetup(inner, signerMock, nil)
		defer srv.Close()

		var buf bytes.Buffer
		_, err := m.Get(ctx, "blue", "1.0.0", release.NopProgressWriter(&buf))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "data integrity check failed")
	})

	t.Run("write methods return ErrReadOnly", func(t *testing.T) {
		m := NewManager(http.DefaultClient, "http://localhost")

		assert.Equal(t, ErrReadOnly, m.Create(ctx, release.Package{}))
		assert.Equal(t, ErrReadOnly, m.DeletePackage(ctx, "pkg"))
		assert.Equal(t, ErrReadOnly, m.Upload(ctx, release.Bundle{}, nil))
		assert.Equal(t, ErrReadOnly, m.Delete(ctx, "pkg", "1.0.0"))
	})
}
