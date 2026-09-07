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

package issue

import (
	"runtime/debug"
	"strings"
	"time"
)

// Report contains informatin about a suspected or confirmed bug.
type Report struct {
	Package   string
	Version   string
	Author    string
	Subject   string
	Notes     string
	Build     debug.BuildInfo `yaml:"build,omitempty"`
	CreatedAt time.Time       `yaml:"created_at,omitempty"`
	UpdatedAt time.Time       `yaml:"updated_at,omitempty"`
	UpdatedBy string          `yaml:"-"`
	Closed    bool            `yaml:"closed,omitempty"`
	ClosedAt  time.Time       `yaml:"closed_at,omitempty"`
	Metadata  map[string]string
}

// IsInternalLabel can be used for Metadata keys to
// figure out if label is internal.
func IsInternalLabel(k string) bool {
	return strings.HasPrefix(k, "_")
}
