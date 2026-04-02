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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
)

const sha256MetadataKey = "sha256"

// ErrReadOnly is returned by write operations.
var ErrReadOnly = errors.New("cdnrelease: read-only manager does not support write operations")

// downloadResponse mirrors gcsrelease.DownloadResponse without importing it.
type downloadResponse struct {
	Bundle release.Bundle `json:"bundle"`
	URL    string         `json:"url"`
}

// Manager is a read-only release.Manager that talks to a
// gcsrelease.Handler over HTTP and downloads data from signed GCS URLs.
type Manager struct {
	// apiClient is used for API requests to the handler (carries oauth2 token).
	apiClient *http.Client
	// dataClient is used for CDN/signed-URL downloads (no auth needed).
	dataClient *http.Client
	baseURL    string
}

var _ release.Manager = (*Manager)(nil)

// NewManager returns a Manager that reads release data from the
// handler at baseURL. apiClient should carry any necessary auth tokens
// for the API. Data downloads from signed GCS URLs use a plain
// http.DefaultClient since signed URLs require no additional auth.
func NewManager(apiClient *http.Client, baseURL string) *Manager {
	return &Manager{apiClient: apiClient, dataClient: http.DefaultClient, baseURL: baseURL}
}

func (m *Manager) Create(context.Context, release.Package) error {
	return ErrReadOnly
}

func (m *Manager) DeletePackage(context.Context, string) error {
	return ErrReadOnly
}

func (m *Manager) Upload(context.Context, release.Bundle, release.ProgressReader) error {
	return ErrReadOnly
}

func (m *Manager) Delete(context.Context, string, release.Version) error {
	return ErrReadOnly
}

func (m *Manager) GetPackage(ctx context.Context, pkg string) (release.Package, error) {
	var p release.Package
	err := m.getJSON(ctx, m.url("packages", pkg), &p)
	return p, err
}

func (m *Manager) ListPackages(ctx context.Context, filters map[string]string) (iterator.Iterator[release.Package], error) {
	var pkgs []release.Package
	err := m.getJSON(ctx, m.urlWithFilters("packages", filters), &pkgs)
	if err != nil {
		return nil, err
	}
	return iterator.FromSlice(pkgs), nil
}

func (m *Manager) List(ctx context.Context, pkg string, filters map[string]string) (iterator.Iterator[release.Bundle], error) {
	var bundles []release.Bundle
	err := m.getJSON(ctx, m.urlWithFilters("packages/"+pkg+"/bundles", filters), &bundles)
	if err != nil {
		return nil, err
	}
	return iterator.FromSlice(bundles), nil
}

func (m *Manager) Get(ctx context.Context, pkg string, ver release.Version, out release.ProgressWriter) (release.Bundle, error) {
	var resp downloadResponse
	err := m.getJSON(ctx, m.url("packages", pkg, "bundles", string(ver), "download"), &resp)
	if err != nil {
		return release.Bundle{}, err
	}

	if del, ok := out.(release.IsDiscard); ok && del.IsDiscard() {
		return resp.Bundle, nil
	}

	// Download data from signed URL.
	req, err := http.NewRequestWithContext(ctx, "GET", resp.URL, nil)
	if err != nil {
		return release.Bundle{}, fmt.Errorf("cdnrelease: create download request: %w", err)
	}
	dataResp, err := m.dataClient.Do(req)
	if err != nil {
		return release.Bundle{}, fmt.Errorf("cdnrelease: download data: %w", err)
	}
	defer func() { _ = dataResp.Body.Close() }()

	if dataResp.StatusCode != http.StatusOK {
		return release.Bundle{}, fmt.Errorf("cdnrelease: download returned status %d", dataResp.StatusCode)
	}

	hasher := sha256.New()
	tee := io.TeeReader(dataResp.Body, hasher)

	totalSize := dataResp.ContentLength
	out.Progress(0, totalSize, "bytes")
	var totalRead int64
	buf := make([]byte, 32*1024)
	for {
		n, rerr := tee.Read(buf)
		if n > 0 {
			_, werr := out.Write(buf[:n])
			if werr != nil {
				return release.Bundle{}, fmt.Errorf("cdnrelease: write data: %w", werr)
			}
			totalRead += int64(n)
			out.Progress(totalRead, totalSize, "bytes")
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return release.Bundle{}, fmt.Errorf("cdnrelease: read data: %w", rerr)
		}
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	if expected, ok := resp.Bundle.Metadata[sha256MetadataKey]; ok && checksum != expected {
		return release.Bundle{}, fmt.Errorf("cdnrelease: data integrity check failed: expected sha256 %s, got %s", expected, checksum)
	}

	return resp.Bundle, nil
}

func (m *Manager) url(parts ...string) string {
	var b strings.Builder
	b.WriteString(m.baseURL)
	for _, p := range parts {
		b.WriteByte('/')
		b.WriteString(p)
	}
	return b.String()
}

func (m *Manager) urlWithFilters(path string, filters map[string]string) string {
	u := m.baseURL + "/" + path
	if len(filters) == 0 {
		return u
	}
	v := make(url.Values)
	for k, val := range filters {
		v.Set(k, val)
	}
	return u + "?" + v.Encode()
}

func (m *Manager) getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("cdnrelease: create request: %w", err)
	}
	resp, err := m.apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("cdnrelease: http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("cdnrelease: %s: not found", url)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cdnrelease: %s: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
