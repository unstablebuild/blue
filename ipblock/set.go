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

package ipblock

import (
	"cmp"
	"fmt"
	"net/netip"
	"slices"
	"time"
)

// canonical returns p as blocks are keyed: IPv4-mapped prefixes as
// IPv4, with the host bits masked off.
func canonical(p netip.Prefix) (netip.Prefix, error) {
	if !p.IsValid() {
		return netip.Prefix{}, fmt.Errorf("ipblock: invalid prefix %s", p)
	}
	if addr := p.Addr(); addr.Is4In6() && p.Bits() >= 96 {
		p = netip.PrefixFrom(addr.Unmap(), p.Bits()-96)
	}
	return p.Masked(), nil
}

// canonicalAddr returns addr as blocks are matched against it.
func canonicalAddr(addr netip.Addr) (netip.Addr, error) {
	if !addr.IsValid() {
		return netip.Addr{}, fmt.Errorf("ipblock: invalid address %s", addr)
	}
	return addr.Unmap().WithZone(""), nil
}

// prepare validates b and returns it as a Writer stores it.
func prepare(b Block, now time.Time) (Block, error) {
	p, err := canonical(b.Prefix)
	if err != nil {
		return Block{}, err
	}
	b.Prefix = p
	if b.BlockedAt.IsZero() {
		b.BlockedAt = now
	}
	b.BlockedAt = b.BlockedAt.UTC()
	if !b.ExpiresAt.IsZero() {
		if !b.ExpiresAt.After(now) {
			return Block{}, fmt.Errorf("ipblock: block on %s would expire at %s, which is not in the future",
				p, b.ExpiresAt.Format(time.RFC3339))
		}
		b.ExpiresAt = b.ExpiresAt.UTC()
	}
	return b, nil
}

// active reports whether b is in force at now.
func active(b Block, now time.Time) bool {
	return b.ExpiresAt.IsZero() || now.Before(b.ExpiresAt)
}

// blockSet indexes blocks by prefix, for matching addresses. It is
// immutable once built.
type blockSet struct {
	byPrefix map[netip.Prefix]Block
	// lengths4 and lengths6 are the prefix lengths some block has,
	// longest first, so a lookup probes only those and finds the
	// narrowest block first.
	lengths4, lengths6 []int
}

// newBlockSet indexes blocks, whose prefixes must be canonical. A later
// block replaces an earlier one on the same prefix.
func newBlockSet(blocks []Block) *blockSet {
	s := &blockSet{byPrefix: make(map[netip.Prefix]Block, len(blocks))}
	for _, b := range blocks {
		if _, ok := s.byPrefix[b.Prefix]; !ok {
			if b.Prefix.Addr().Is4() {
				s.lengths4 = append(s.lengths4, b.Prefix.Bits())
			} else {
				s.lengths6 = append(s.lengths6, b.Prefix.Bits())
			}
		}
		s.byPrefix[b.Prefix] = b
	}
	for _, lengths := range []*[]int{&s.lengths4, &s.lengths6} {
		slices.Sort(*lengths)
		*lengths = slices.Compact(*lengths)
		slices.Reverse(*lengths)
	}
	return s
}

// lookup returns the narrowest block in force at now that covers addr,
// which must be canonical.
func (s *blockSet) lookup(addr netip.Addr, now time.Time) (Block, bool) {
	lengths := s.lengths6
	if addr.Is4() {
		lengths = s.lengths4
	}
	for _, bits := range lengths {
		p := netip.PrefixFrom(addr, bits).Masked()
		if b, ok := s.byPrefix[p]; ok && active(b, now) {
			return b, true
		}
	}
	return Block{}, false
}

// list returns the blocks in force at now, ordered by prefix.
func (s *blockSet) list(now time.Time) []Block {
	ret := make([]Block, 0, len(s.byPrefix))
	for _, b := range s.byPrefix {
		if active(b, now) {
			ret = append(ret, b)
		}
	}
	slices.SortFunc(ret, func(a, b Block) int {
		if c := a.Prefix.Addr().Compare(b.Prefix.Addr()); c != 0 {
			return c
		}
		return cmp.Compare(a.Prefix.Bits(), b.Prefix.Bits())
	})
	return ret
}
