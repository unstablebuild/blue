// Copyright 2026 Unstable Build, LLC.
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

package html

import (
	"net/http"

	htmlcomp "github.com/unstablebuild/blue/tui/component/html"
	"github.com/unstablebuild/blue/tui/component/markdown"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// Option configures a [Handler].
type Option func(*Handler)

// WithHTTPClient sets the HTTP client used for fetching HTML content.
// Defaults to [http.DefaultClient].
func WithHTTPClient(client *http.Client) Option {
	return func(h *Handler) {
		h.httpClient = client
	}
}

// WithMarkdownConfig sets the markdown rendering configuration used
// when converting fetched HTML content. Defaults to
// [markdown.DefaultConfig].
func WithMarkdownConfig(cfg markdown.Config) Option {
	return func(h *Handler) {
		h.mdCfg = cfg
	}
}

// WithSelectionAttrs sets the attributes used to highlight selected
// text. Defaults to [tcell.AttrReverse].
func WithSelectionAttrs(attrs term.Attributes) Option {
	return func(h *Handler) {
		h.selectionAttrs = attrs
	}
}

// WithComponentOptions appends additional options that are forwarded
// to every [htmlcomp.Component] created by the handler.
func WithComponentOptions(opts ...htmlcomp.Option) Option {
	return func(h *Handler) {
		h.compOpts = append(h.compOpts, opts...)
	}
}
