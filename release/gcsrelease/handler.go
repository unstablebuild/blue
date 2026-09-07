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
	"encoding/json"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
)

// DownloadResponse is the JSON payload for the download endpoint.
type DownloadResponse struct {
	Bundle release.Bundle `json:"bundle"`
	URL    string         `json:"url"`
}

// NewHandler returns an http.Handler serving release metadata and
// signed download URLs.
func NewHandler(m release.Manager, signer Signer) http.Handler {
	h := &handler{m: m, signer: signer}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /packages", h.listPackages)
	mux.HandleFunc("GET /packages/{name}", h.getPackage)
	mux.HandleFunc("GET /packages/{name}/bundles", h.listBundles)
	mux.HandleFunc("GET /packages/{name}/bundles/{version}", h.getBundle)
	mux.HandleFunc("GET /packages/{name}/bundles/{version}/download", h.downloadBundle)
	return mux
}

type handler struct {
	m      release.Manager
	signer Signer
}

func (h *handler) listPackages(w http.ResponseWriter, r *http.Request) {
	filters := queryFilters(r)
	it, err := h.m.ListPackages(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, "listPackages",
			logrus.Fields{"filters": filters})
		return
	}
	pkgs, err := iterator.ToSlice(r.Context(), it)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, "listPackages",
			logrus.Fields{"filters": filters})
		return
	}
	writeJSON(w, http.StatusOK, pkgs)
}

func (h *handler) getPackage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pkg, err := h.m.GetPackage(r.Context(), name)
	if err != nil {
		writeError(w, statusFromError(err), err, "getPackage",
			logrus.Fields{"package": name})
		return
	}
	writeJSON(w, http.StatusOK, pkg)
}

func (h *handler) listBundles(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	filters := queryFilters(r)
	it, err := h.m.List(r.Context(), name, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, "listBundles",
			logrus.Fields{"package": name, "filters": filters})
		return
	}
	bundles, err := iterator.ToSlice(r.Context(), it)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, "listBundles",
			logrus.Fields{"package": name, "filters": filters})
		return
	}
	writeJSON(w, http.StatusOK, bundles)
}

func (h *handler) getBundle(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := release.Version(r.PathValue("version"))
	bundle, err := h.m.Get(r.Context(), name, version, release.NopProgressWriter(io.Discard))
	if err != nil {
		writeError(w, statusFromError(err), err, "getBundle",
			logrus.Fields{"package": name, "version": string(version)})
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

func (h *handler) downloadBundle(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := release.Version(r.PathValue("version"))
	bundle, err := h.m.Get(r.Context(), name, version, release.NopProgressWriter(io.Discard))
	if err != nil {
		writeError(w, statusFromError(err), err, "downloadBundle", logrus.Fields{
			"package": name,
			"version": string(version),
		})
		return
	}
	url, err := h.signer.SignedDownloadURL(r.Context(), name, version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, "downloadBundle", logrus.Fields{
			"package": name,
			"version": string(version),
		})
		return
	}
	logrus.WithFields(logrus.Fields{
		"package": name,
		"version": string(version),
		"route":   "downloadBundle",
		"url":     url,
	}).Debug("signed release download URL")
	writeJSON(w, http.StatusOK, DownloadResponse{Bundle: bundle, URL: url})
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

func statusFromError(err error) int {
	if isNotFound(err) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	// document.ErrNotFound ("document not found") is the sentinel used by docrelease
	return err.Error() == "document not found"
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error, route string, fields logrus.Fields) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["route"] = route
	fields["status"] = code
	entry := logrus.WithFields(fields).WithError(err)
	// 4xx responses are client mistakes (unknown package/bundle), not
	// service faults; logging them at ERROR floods dashboards and hides
	// real 5xx failures.
	if code >= http.StatusInternalServerError {
		entry.Error("release handler failure")
	} else {
		entry.Warn("release handler failure")
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func init() {
	// Ensure Manager satisfies Signer at compile time.
	var _ Signer = (*Manager)(nil)

	// Ensure Manager satisfies release.Manager at compile time.
	var _ release.Manager = (*Manager)(nil)
}
