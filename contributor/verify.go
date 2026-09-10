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

import "fmt"

// Statement is a non-sensitive export of a closed round together with
// the policy it was computed under. It contains everything needed to
// recompute the allocation: frozen aggregate inputs and per-account
// credits, but no receipts, payee identities or provider references.
type Statement struct {
	Program Program
	Round   Round
}

// VerifyRound recomputes the allocation of a closed round from its
// frozen inputs and returns an error describing the first discrepancy
// found, or nil if the round's published results are reproducible.
func VerifyRound(s Statement) error {
	p, r := s.Program, s.Round
	if err := p.Validate(); err != nil {
		return err
	}
	if err := r.Month.Validate(); err != nil {
		return err
	}
	if r.PolicyVersion != p.Version {
		return fmt.Errorf(
			"round %s: policy version %q does not match program %q",
			r.Month, r.PolicyVersion, p.Version)
	}
	if r.CalculationVersion != CalculationVersion {
		return fmt.Errorf(
			"round %s: calculation version %d not supported by engine %d",
			r.Month, r.CalculationVersion, CalculationVersion)
	}
	if r.Currency != p.Currency {
		return fmt.Errorf(
			"round %s: currency %q does not match program %q",
			r.Month, r.Currency, p.Currency)
	}

	pool := r.CarryforwardCents + PoolShareCents(p.PoolBps, r.NetReceiptsCents)
	if pool != r.PoolCents {
		return fmt.Errorf(
			"round %s: recomputed pool %d does not match published %d",
			r.Month, pool, r.PoolCents)
	}

	credits := make(map[string]int64, len(r.Entitlements))
	var totalCredits int64
	for _, e := range r.Entitlements {
		if _, ok := credits[e.Account]; ok {
			return fmt.Errorf(
				"round %s: duplicate entitlement account %q",
				r.Month, e.Account)
		}
		credits[e.Account] = e.Credits
		totalCredits += e.Credits
	}
	if totalCredits != r.TotalCredits {
		return fmt.Errorf(
			"round %s: entitlement credits sum %d does not match "+
				"published denominator %d",
			r.Month, totalCredits, r.TotalCredits)
	}

	allocatable := pool
	if allocatable < 0 {
		allocatable = 0
	}
	ents, err := Allocate(allocatable, credits)
	if err != nil {
		return fmt.Errorf("round %s: %v", r.Month, err)
	}
	if len(ents) != len(r.Entitlements) {
		return fmt.Errorf(
			"round %s: recomputed %d entitlements, published %d",
			r.Month, len(ents), len(r.Entitlements))
	}
	for i, e := range ents {
		pub := r.Entitlements[i]
		if e != pub {
			return fmt.Errorf(
				"round %s: entitlement %q: recomputed %d cents "+
					"(%d credits), published %d cents (%d credits)",
				r.Month, e.Account, e.AmountCents, e.Credits,
				pub.AmountCents, pub.Credits)
		}
	}

	if got := pool - r.AllocatedCents(); got != r.UnallocatedCents {
		return fmt.Errorf(
			"round %s: recomputed unallocated %d does not match "+
				"published %d",
			r.Month, got, r.UnallocatedCents)
	}
	return nil
}
