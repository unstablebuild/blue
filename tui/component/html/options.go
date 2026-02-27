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

	"github.com/unstablebuild/blue/tui/component/markdown"
)

type config struct {
	mdCfg      markdown.Config
	httpClient *http.Client
}

// Option configures a [Component].
type Option func(*config)

// WithMarkdownConfig sets the markdown rendering configuration.
func WithMarkdownConfig(cfg markdown.Config) Option {
	return func(c *config) {
		c.mdCfg = cfg
	}
}

// WithHTTPClient sets the HTTP client used for fetching content.
func WithHTTPClient(client *http.Client) Option {
	return func(c *config) {
		c.httpClient = client
	}
}
