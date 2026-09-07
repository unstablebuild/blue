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

package secretmanager

import "time"

// State can be one of enabled, disabled or destroyed.
type State string

const (
	StateEnabled   State = "enabled"
	StateDisabled  State = "disabled"
	StateDestroyed State = "destroyed"
	StateUnknown   State = "unknown"
)

// SecretVersion represents a securely stored secret version.
type SecretVersion struct {
	ID        string
	Version   string
	State     State
	CreatedAt time.Time `json:"created_at,omitempty"`
	Payload   []byte
}

// Secret represents a logical secret whose values can be accessed.
type Secret struct {
	ID          string
	CreatedAt   time.Time
	Annotations map[string]string
}
