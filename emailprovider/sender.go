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
