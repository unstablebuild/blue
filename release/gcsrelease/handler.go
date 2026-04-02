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
	logrus.WithFields(fields).WithError(err).Error("release handler failure")
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func init() {
	// Ensure Manager satisfies Signer at compile time.
	var _ Signer = (*Manager)(nil)

	// Ensure Manager satisfies release.Manager at compile time.
	var _ release.Manager = (*Manager)(nil)
}
