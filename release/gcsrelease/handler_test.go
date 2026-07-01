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
	"context"
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

var errNotFound = errors.New("document not found")

type mockManager struct {
	createFn        func(ctx context.Context, p release.Package) error
	deletePackageFn func(ctx context.Context, pkg string) error
	getPackageFn    func(ctx context.Context, pkg string) (release.Package, error)
	listPackagesFn  func(ctx context.Context, filters map[string]string) (iterator.Iterator[release.Package], error)
	uploadFn        func(ctx context.Context, b release.Bundle, r release.ProgressReader) error
	getFn           func(ctx context.Context, pkg string, ver release.Version, w release.ProgressWriter) (release.Bundle, error)
	deleteFn        func(ctx context.Context, pkg string, ver release.Version) error
	listFn          func(ctx context.Context, pkg string, filters map[string]string) (iterator.Iterator[release.Bundle], error)
}

func (m *mockManager) Create(ctx context.Context, p release.Package) error {
	return m.createFn(ctx, p)
}
func (m *mockManager) UpdatePackageMetadata(ctx context.Context, pkg string, metadata map[string]string) error {
	return nil
}
func (m *mockManager) DeletePackage(ctx context.Context, pkg string) error {
	return m.deletePackageFn(ctx, pkg)
}
func (m *mockManager) GetPackage(ctx context.Context, pkg string) (release.Package, error) {
	return m.getPackageFn(ctx, pkg)
}
func (m *mockManager) ListPackages(ctx context.Context, filters map[string]string) (iterator.Iterator[release.Package], error) {
	return m.listPackagesFn(ctx, filters)
}
func (m *mockManager) Upload(ctx context.Context, b release.Bundle, r release.ProgressReader) error {
	return m.uploadFn(ctx, b, r)
}
func (m *mockManager) Get(ctx context.Context, pkg string, ver release.Version, w release.ProgressWriter) (release.Bundle, error) {
	return m.getFn(ctx, pkg, ver, w)
}
func (m *mockManager) Delete(ctx context.Context, pkg string, ver release.Version) error {
	return m.deleteFn(ctx, pkg, ver)
}
func (m *mockManager) List(ctx context.Context, pkg string, filters map[string]string) (iterator.Iterator[release.Bundle], error) {
	return m.listFn(ctx, pkg, filters)
}

type mockSigner struct {
	signFn func(ctx context.Context, pkg string, version release.Version) (string, error)
}

func (s *mockSigner) SignedDownloadURL(ctx context.Context, pkg string, version release.Version) (string, error) {
	return s.signFn(ctx, pkg, version)
}

func TestHandler(t *testing.T) {
	t.Run("list packages", func(t *testing.T) {
		pkgs := []release.Package{
			{Name: "alpha", Latest: "1.0.0"},
			{Name: "beta", Latest: "2.0.0"},
		}
		m := &mockManager{
			listPackagesFn: func(_ context.Context, filters map[string]string) (iterator.Iterator[release.Package], error) {
				return iterator.FromSlice(pkgs), nil
			},
		}
		h := NewHandler(m, nil)

		req := httptest.NewRequest("GET", "/packages", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var got []release.Package
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
		assert.Equal(t, pkgs, got)
	})

	t.Run("list packages with filters", func(t *testing.T) {
		var capturedFilters map[string]string
		m := &mockManager{
			listPackagesFn: func(_ context.Context, filters map[string]string) (iterator.Iterator[release.Package], error) {
				capturedFilters = filters
				return iterator.FromSlice[release.Package](nil), nil
			},
		}
		h := NewHandler(m, nil)

		req := httptest.NewRequest("GET", "/packages?os=linux&arch=amd64", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]string{"os": "linux", "arch": "amd64"}, capturedFilters)
	})

	t.Run("list packages empty", func(t *testing.T) {
		m := &mockManager{
			listPackagesFn: func(_ context.Context, _ map[string]string) (iterator.Iterator[release.Package], error) {
				return iterator.FromSlice[release.Package](nil), nil
			},
		}
		h := NewHandler(m, nil)

		req := httptest.NewRequest("GET", "/packages", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var got []release.Package
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
		assert.Empty(t, got)
	})

	t.Run("get package found", func(t *testing.T) {
		pkg := release.Package{Name: "blue", Latest: "1.2.3"}
		m := &mockManager{
			getPackageFn: func(_ context.Context, name string) (release.Package, error) {
				if name == "blue" {
					return pkg, nil
				}
				return release.Package{}, errNotFound
			},
		}
		h := NewHandler(m, nil)

		req := httptest.NewRequest("GET", "/packages/blue", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var got release.Package
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
		assert.Equal(t, pkg, got)
	})

	t.Run("get package not found", func(t *testing.T) {
		m := &mockManager{
			getPackageFn: func(_ context.Context, _ string) (release.Package, error) {
				return release.Package{}, errNotFound
			},
		}
		h := NewHandler(m, nil)

		req := httptest.NewRequest("GET", "/packages/nonexistent", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("list bundles", func(t *testing.T) {
		bundles := []release.Bundle{
			{Package: "blue", Version: "1.0.0"},
			{Package: "blue", Version: "1.0.1"},
		}
		m := &mockManager{
			listFn: func(_ context.Context, pkg string, _ map[string]string) (iterator.Iterator[release.Bundle], error) {
				if pkg == "blue" {
					return iterator.FromSlice(bundles), nil
				}
				return iterator.FromSlice[release.Bundle](nil), nil
			},
		}
		h := NewHandler(m, nil)

		req := httptest.NewRequest("GET", "/packages/blue/bundles", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var got []release.Bundle
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
		assert.Equal(t, bundles, got)
	})

	t.Run("list bundles with filters", func(t *testing.T) {
		var capturedFilters map[string]string
		m := &mockManager{
			listFn: func(_ context.Context, _ string, filters map[string]string) (iterator.Iterator[release.Bundle], error) {
				capturedFilters = filters
				return iterator.FromSlice[release.Bundle](nil), nil
			},
		}
		h := NewHandler(m, nil)

		req := httptest.NewRequest("GET", "/packages/blue/bundles?Metadata.os=linux", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]string{"Metadata.os": "linux"}, capturedFilters)
	})

	t.Run("get bundle found", func(t *testing.T) {
		bundle := release.Bundle{
			Package:  "blue",
			Version:  "1.0.0",
			Metadata: map[string]string{"sha256": "abc123"},
		}
		m := &mockManager{
			getFn: func(_ context.Context, pkg string, ver release.Version, _ release.ProgressWriter) (release.Bundle, error) {
				if pkg == "blue" && ver == "1.0.0" {
					return bundle, nil
				}
				return release.Bundle{}, errNotFound
			},
		}
		h := NewHandler(m, nil)

		req := httptest.NewRequest("GET", "/packages/blue/bundles/1.0.0", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var got release.Bundle
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
		assert.Equal(t, bundle, got)
	})

	t.Run("get bundle not found", func(t *testing.T) {
		m := &mockManager{
			getFn: func(_ context.Context, _ string, _ release.Version, _ release.ProgressWriter) (release.Bundle, error) {
				return release.Bundle{}, errNotFound
			},
		}
		h := NewHandler(m, nil)

		req := httptest.NewRequest("GET", "/packages/blue/bundles/9.9.9", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("download bundle success", func(t *testing.T) {
		bundle := release.Bundle{
			Package:  "blue",
			Version:  "1.0.0",
			Metadata: map[string]string{"sha256": "abc123"},
		}
		m := &mockManager{
			getFn: func(_ context.Context, pkg string, ver release.Version, _ release.ProgressWriter) (release.Bundle, error) {
				return bundle, nil
			},
		}
		signer := &mockSigner{
			signFn: func(_ context.Context, pkg string, ver release.Version) (string, error) {
				return "https://storage.example.com/signed/" + pkg + "/" + string(ver), nil
			},
		}
		h := NewHandler(m, signer)

		req := httptest.NewRequest("GET", "/packages/blue/bundles/1.0.0/download", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var got DownloadResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
		assert.Equal(t, bundle, got.Bundle)
		assert.Equal(t, "https://storage.example.com/signed/blue/1.0.0", got.URL)
	})

	t.Run("download bundle not found", func(t *testing.T) {
		m := &mockManager{
			getFn: func(_ context.Context, _ string, _ release.Version, _ release.ProgressWriter) (release.Bundle, error) {
				return release.Bundle{}, errNotFound
			},
		}
		h := NewHandler(m, &mockSigner{})

		req := httptest.NewRequest("GET", "/packages/blue/bundles/9.9.9/download", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("download bundle signer error", func(t *testing.T) {
		bundle := release.Bundle{Package: "blue", Version: "1.0.0"}
		m := &mockManager{
			getFn: func(_ context.Context, _ string, _ release.Version, _ release.ProgressWriter) (release.Bundle, error) {
				return bundle, nil
			},
		}
		signer := &mockSigner{
			signFn: func(_ context.Context, _ string, _ release.Version) (string, error) {
				return "", errors.New("signing failed")
			},
		}
		h := NewHandler(m, signer)

		req := httptest.NewRequest("GET", "/packages/blue/bundles/1.0.0/download", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

var _ release.Manager = (*mockManager)(nil)
var _ Signer = (*mockSigner)(nil)

// Verify handler uses Discard writer for Get calls.
func TestHandlerUsesDiscardWriter(t *testing.T) {
	var writerWasDiscard bool
	m := &mockManager{
		getFn: func(_ context.Context, _ string, _ release.Version, w release.ProgressWriter) (release.Bundle, error) {
			if d, ok := w.(release.IsDiscard); ok {
				writerWasDiscard = d.IsDiscard()
			}
			return release.Bundle{Package: "test", Version: "1.0.0"}, nil
		},
	}
	signer := &mockSigner{
		signFn: func(_ context.Context, _ string, _ release.Version) (string, error) {
			return "https://example.com", nil
		},
	}
	h := NewHandler(m, signer)

	req := httptest.NewRequest("GET", "/packages/test/bundles/1.0.0/download", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, writerWasDiscard, "handler should pass Discard writer to Get")

	// suppress unused import for io
	_ = io.Discard
}
