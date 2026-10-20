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
	"net/netip"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// cacheLoadTimeout bounds one load of the cached blocks from their
// source.
const cacheLoadTimeout = 30 * time.Second

// NewCache returns a Reader that answers from an in-memory copy of the
// blocks src lists, so that checking an address costs no round trip to
// src. The copy is loaded on the first call and reloaded once it is ttl
// old, which bounds how long a change to src takes to show:
//
//   - The first call waits for the load. If the load fails, the error
//     is returned until ttl has passed, rather than every call retrying
//     against a source that is down.
//   - Once loaded, calls never wait: a reload runs in the background
//     while the current copy keeps answering.
//   - A failed reload keeps the current copy, so a source outage
//     leaves the blocks it last listed in force rather than lifting
//     them all. Blocks still lapse at their ExpiresAt meanwhile.
//
// It panics when src is nil or ttl is not positive.
func NewCache(src Reader, ttl time.Duration) Reader {
	if src == nil {
		panic("ipblock.NewCache: src must not be nil")
	}
	if ttl <= 0 {
		panic("ipblock.NewCache: ttl must be positive")
	}
	return newCache(src, ttl, time.Now)
}

type cache struct {
	src Reader
	ttl time.Duration
	now func() time.Time

	mu sync.Mutex
	// set is the last copy loaded; nil until a load succeeds.
	set *blockSet
	// err is the error of the last load when set is nil.
	err error
	// attempted is when the last load started.
	attempted time.Time
	// loading is closed when the load in flight finishes; nil when no
	// load is in flight.
	loading chan struct{}
}

func newCache(src Reader, ttl time.Duration, now func() time.Time) *cache {
	return &cache{src: src, ttl: ttl, now: now}
}

func (c *cache) Check(ctx context.Context, addr netip.Addr) (Block, bool, error) {
	addr, err := canonicalAddr(addr)
	if err != nil {
		return Block{}, false, err
	}
	set, err := c.current(ctx)
	if err != nil {
		return Block{}, false, err
	}
	b, ok := set.lookup(addr, c.now())
	return b, ok, nil
}

func (c *cache) List(ctx context.Context) ([]Block, error) {
	set, err := c.current(ctx)
	if err != nil {
		return nil, err
	}
	return set.list(c.now()), nil
}

// current returns the copy to answer from, starting a load when the
// copy is due and waiting for it only when there is no copy yet.
func (c *cache) current(ctx context.Context) (*blockSet, error) {
	c.mu.Lock()
	if c.loading == nil && c.now().Sub(c.attempted) >= c.ttl {
		c.loading = make(chan struct{})
		c.attempted = c.now()
		// The load outlives the call that started it: other calls may
		// be waiting for it, and abandoning it would leave them none.
		go c.load(context.WithoutCancel(ctx), c.loading)
	}
	set, loading, err := c.set, c.loading, c.err
	c.mu.Unlock()

	if set != nil {
		return set, nil
	}
	if loading == nil {
		return nil, err
	}
	select {
	case <-loading:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.set == nil {
		return nil, c.err
	}
	return c.set, nil
}

func (c *cache) load(ctx context.Context, done chan struct{}) {
	defer close(done)
	ctx, cancel := context.WithTimeout(ctx, cacheLoadTimeout)
	defer cancel()
	blocks, err := c.src.List(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.loading = nil
	if err != nil {
		c.err = err
		if c.set != nil {
			log.WithError(err).Warn("ipblock: reloading blocks failed, keeping the blocks last loaded")
		}
		return
	}
	c.set, c.err = newBlockSet(blocks), nil
}
