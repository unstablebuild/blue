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

package markdown

import (
	"net/url"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// Option configures a Handler.
type Option func(*Handler)

// WithOnLinkClick sets a callback for when a link is clicked.
// The callback returns true if it handled the link, false otherwise.
// If the callback returns false (or is not set), clicking local anchors
// (URLs starting with #) will scroll to the corresponding header.
func WithOnLinkClick(fn func(*url.URL) bool) Option {
	return func(h *Handler) {
		h.onLinkClick = fn
	}
}

// WithSelectionAttrs sets the attributes used to highlight selected text.
// The attributes are unioned with existing cell attributes.
// Defaults to term.Attributes with AttrReverse.
func WithSelectionAttrs(attrs term.Attributes) Option {
	return func(h *Handler) {
		h.selectionAttrs = attrs
	}
}
