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

// Package emailprovider defines provider-neutral email sending types.
package emailprovider

import (
	"context"
	"net/mail"
)

// Address identifies an email mailbox and its optional display name.
type Address struct {
	Name  string
	Email string
}

// String formats the address as an RFC 5322 mailbox.
func (a Address) String() string {
	return (&mail.Address{Name: a.Name, Address: a.Email}).String()
}

// Message is one private email for one recipient. A Sender may batch multiple
// messages internally, but it must not expose recipients to one another.
type Message struct {
	Recipient Address
	Subject   string
	HTMLBody  string
	TextBody  string
	Metadata  map[string]string
}

// Status describes whether a provider accepted a message for delivery.
type Status string

const (
	// StatusAccepted means the provider accepted the message. It does not imply delivery.
	StatusAccepted Status = "accepted"
	// StatusRejected means the provider rejected the message before delivery.
	StatusRejected Status = "rejected"
	// StatusUnknown means the provider could not determine whether it accepted the message.
	StatusUnknown Status = "unknown"
)

// Result describes the outcome for the message at the same index passed to Send.
type Result struct {
	Recipient  Address
	Status     Status
	ProviderID string
	Err        error
}

// Sender submits email in bulk. Results must correspond by index to messages.
// A non-nil method error indicates that at least one result may be incomplete;
// callers should inspect both the returned results and error.
type Sender interface {
	Send(ctx context.Context, messages []Message) ([]Result, error)
}
