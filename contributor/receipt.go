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

package contributor

import (
	"errors"
	"fmt"
	"time"
)

// AdjustmentKind enumerates the permitted deductions from a receipt's
// gross amount. Only enumerated kinds may reduce the covered pool base.
type AdjustmentKind string

const (
	// AdjustmentTax covers sales tax, VAT and similar collected taxes.
	AdjustmentTax AdjustmentKind = "tax"
	// AdjustmentRefund covers refunds and chargebacks.
	AdjustmentRefund AdjustmentKind = "refund"
	// AdjustmentPassThrough covers amounts collected on behalf of third
	// parties that are not program revenue.
	AdjustmentPassThrough AdjustmentKind = "pass-through"
)

// Adjustment is a single deduction from a receipt's gross amount.
type Adjustment struct {
	// Kind is the deduction category.
	Kind AdjustmentKind
	// AmountCents is the deducted amount in cents. Always non-negative;
	// it is subtracted from the gross.
	AmountCents int64
	// Description optionally explains the adjustment.
	Description string
}

// Receipt records covered revenue attributed to a month. Receipts are
// imported from the billing system with an opaque source reference that
// doubles as their identity, making imports idempotent.
type Receipt struct {
	// Source is the opaque, unique reference to the originating billing
	// record (e.g. an invoice identifier). It is the Receipt's identity.
	Source string

	// Month is the round the receipt is attributed to.
	Month Month

	// GrossCents is the gross amount in cents.
	GrossCents int64

	// Adjustments enumerates deductions from the gross amount.
	Adjustments []Adjustment

	// Currency is the ISO 4217 currency code, e.g. "usd". It must match
	// the program currency for the receipt to be covered.
	Currency string

	// Evidence is an opaque reference to supporting documentation.
	Evidence string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NetCents returns the gross amount minus all adjustments.
func (r Receipt) NetCents() int64 {
	net := r.GrossCents
	for _, a := range r.Adjustments {
		net -= a.AmountCents
	}
	return net
}

// Validate returns an error if the Receipt is not well formed.
func (r Receipt) Validate() error {
	if r.Source == "" {
		return errors.New("invalid receipt: missing source")
	}
	if err := r.Month.Validate(); err != nil {
		return fmt.Errorf("invalid receipt %q: %v", r.Source, err)
	}
	if r.GrossCents < 0 {
		return fmt.Errorf("invalid receipt %q: negative gross", r.Source)
	}
	if r.Currency == "" {
		return fmt.Errorf("invalid receipt %q: missing currency", r.Source)
	}
	for _, a := range r.Adjustments {
		switch a.Kind {
		case AdjustmentTax, AdjustmentRefund, AdjustmentPassThrough:
		default:
			return fmt.Errorf(
				"invalid receipt %q: unknown adjustment kind %q",
				r.Source, a.Kind)
		}
		if a.AmountCents < 0 {
			return fmt.Errorf(
				"invalid receipt %q: negative adjustment", r.Source)
		}
	}
	return nil
}
