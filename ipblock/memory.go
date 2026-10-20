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
	"context"
	"maps"
	"net/netip"
	"slices"
	"sync"
	"time"
)

// NewMemoryList returns a List kept in process memory. Its
// blocks are not shared with other processes and do not outlive this
// one.
func NewMemoryList() List {
	return newMemoryList(time.Now)
}

type memoryList struct {
	now func() time.Time

	mu     sync.RWMutex
	blocks map[netip.Prefix]Block
	set    *blockSet
}

func newMemoryList(now func() time.Time) *memoryList {
	return &memoryList{
		now:    now,
		blocks: make(map[netip.Prefix]Block),
		set:    newBlockSet(nil),
	}
}

func (m *memoryList) Block(_ context.Context, b Block) error {
	b, err := prepare(b, m.now())
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks[b.Prefix] = b
	m.reindex()
	return nil
}

func (m *memoryList) Unblock(_ context.Context, p netip.Prefix) error {
	p, err := canonical(p)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blocks[p]
	if !ok {
		return ErrNotBlocked
	}
	delete(m.blocks, p)
	m.reindex()
	if !active(b, m.now()) {
		return ErrNotBlocked
	}
	return nil
}

// reindex rebuilds the lookup index; m.mu must be held for writing.
// Blocks change rarely and are checked often, so the index is rebuilt
// whole rather than kept up to date.
func (m *memoryList) reindex() {
	m.set = newBlockSet(slices.Collect(maps.Values(m.blocks)))
}

func (m *memoryList) Check(_ context.Context, addr netip.Addr) (Block, bool, error) {
	addr, err := canonicalAddr(addr)
	if err != nil {
		return Block{}, false, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.set.lookup(addr, m.now())
	return b, ok, nil
}

func (m *memoryList) List(context.Context) ([]Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.set.list(m.now()), nil
}
