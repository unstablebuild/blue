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

// Package contributor implements the core of the Unstable Build
// contributor program: domain types, storage-agnostic stores and a
// deterministic allocation engine.
//
// The package is intentionally free of provider dependencies (no cloud,
// payment or identity SDKs) so that the entire allocation logic is public
// and independently auditable. Monetary amounts are always integer cents;
// credits are allocation weights, never currency.
package contributor

import (
	"fmt"
	"time"
)

// monthLayout is the canonical string representation of a Month.
const monthLayout = "2006-01"

// Month identifies a monthly allocation round in "YYYY-MM" form.
// The representation sorts lexicographically in chronological order,
// which makes it usable directly in document store filters.
type Month string

// MonthOf returns the Month containing the given time, in UTC.
func MonthOf(t time.Time) Month {
	return Month(t.UTC().Format(monthLayout))
}

// ParseMonth parses a Month in "YYYY-MM" form.
func ParseMonth(s string) (Month, error) {
	m := Month(s)
	if err := m.Validate(); err != nil {
		return "", err
	}
	return m, nil
}

// Validate returns an error if the Month is not in "YYYY-MM" form.
func (m Month) Validate() error {
	t, err := time.Parse(monthLayout, string(m))
	if err != nil {
		return fmt.Errorf("invalid month %q: expected YYYY-MM", string(m))
	}
	if MonthOf(t) != m {
		return fmt.Errorf("invalid month %q: not canonical", string(m))
	}
	return nil
}

// Time returns the first instant of the month in UTC.
// It panics if the Month is invalid; call Validate first on
// untrusted input.
func (m Month) Time() time.Time {
	t, err := time.Parse(monthLayout, string(m))
	if err != nil {
		panic(err)
	}
	return t
}

// Add returns the Month n months after m. Negative values of n are
// permitted and move backwards in time.
func (m Month) Add(n int) Month {
	return MonthOf(m.Time().AddDate(0, n, 0))
}

// Next returns the month immediately after m.
func (m Month) Next() Month { return m.Add(1) }

// Prev returns the month immediately before m.
func (m Month) Prev() Month { return m.Add(-1) }

// Before reports whether m is strictly earlier than o.
func (m Month) Before(o Month) bool { return m < o }
