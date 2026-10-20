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
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSource is a Reader whose List the test controls and counts.
type fakeSource struct {
	mu     sync.Mutex
	blocks []Block
	err    error
	calls  int
	// gate, when set, holds every List until it is closed.
	gate chan struct{}
}

func (s *fakeSource) set(err error, blocks ...Block) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocks, s.err = blocks, err
}

func (s *fakeSource) hold() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gate = make(chan struct{})
	return s.gate
}

func (s *fakeSource) listCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *fakeSource) Check(context.Context, netip.Addr) (Block, bool, error) {
	panic("the cache must answer from its own copy")
}

func (s *fakeSource) List(ctx context.Context) ([]Block, error) {
	s.mu.Lock()
	s.calls++
	gate := s.gate
	s.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
	// Like a real store, a List whose context is done fails.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.blocks), s.err
}

const testTTL = time.Minute

func newTestCache(src *fakeSource) (*cache, *testClock) {
	clock := newTestClock()
	return newCache(src, testTTL, clock.Now), clock
}

// waitIdle waits for the load in flight, if any, to finish.
func waitIdle(t *testing.T, c *cache) {
	t.Helper()
	require.Eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.loading == nil
	}, 5*time.Second, time.Millisecond)
}

func block(prefix, reason string) Block {
	return Block{Prefix: netip.MustParsePrefix(prefix), Reason: reason}
}

// checkReason returns the reason of the block covering addr, or "" when
// there is none.
func checkReason(t *testing.T, c Reader, addr string) string {
	t.Helper()
	b, ok, err := c.Check(context.Background(), netip.MustParseAddr(addr))
	require.NoError(t, err)
	if !ok {
		return ""
	}
	return b.Reason
}

func TestCache_AnswersFromMemoryWithinTTL(t *testing.T) {
	src := &fakeSource{}
	src.set(nil, block("203.0.113.0/24", "spam"))
	c, clock := newTestCache(src)

	for range 100 {
		assert.Equal(t, "spam", checkReason(t, c, "203.0.113.9"))
		assert.Empty(t, checkReason(t, c, "198.51.100.1"))
	}
	clock.Advance(testTTL - time.Second)
	assert.Equal(t, "spam", checkReason(t, c, "203.0.113.9"))

	blocks, err := c.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, blocks, 1)
	assert.Equal(t, 1, src.listCalls())
}

func TestCache_ReloadsInTheBackgroundOnceStale(t *testing.T) {
	src := &fakeSource{}
	src.set(nil, block("203.0.113.0/24", "old"))
	c, clock := newTestCache(src)
	require.Equal(t, "old", checkReason(t, c, "203.0.113.9"))

	src.set(nil, block("203.0.113.0/24", "new"))
	release := src.hold()
	clock.Advance(testTTL)
	assert.Equal(t, "old", checkReason(t, c, "203.0.113.9"), "a stale copy still answers")
	require.Eventually(t, func() bool { return src.listCalls() == 2 },
		5*time.Second, time.Millisecond, "the stale copy is reloaded")

	// While the reload is held, calls keep answering from the old copy
	// and do not start another reload.
	for range 10 {
		assert.Equal(t, "old", checkReason(t, c, "203.0.113.9"))
	}
	assert.Equal(t, 2, src.listCalls())

	close(release)
	waitIdle(t, c)
	assert.Equal(t, "new", checkReason(t, c, "203.0.113.9"))
	assert.Equal(t, 2, src.listCalls())
}

func TestCache_FailedReloadKeepsTheLastCopy(t *testing.T) {
	src := &fakeSource{}
	src.set(nil, block("203.0.113.0/24", "spam"))
	c, clock := newTestCache(src)
	require.Equal(t, "spam", checkReason(t, c, "203.0.113.9"))

	src.set(errors.New("store down"))
	clock.Advance(testTTL)
	assert.Equal(t, "spam", checkReason(t, c, "203.0.113.9"))
	waitIdle(t, c)
	assert.Equal(t, "spam", checkReason(t, c, "203.0.113.9"), "an outage does not lift blocks")

	src.set(nil)
	clock.Advance(testTTL)
	checkReason(t, c, "203.0.113.9")
	waitIdle(t, c)
	assert.Empty(t, checkReason(t, c, "203.0.113.9"), "the recovered store is read again")
	assert.Equal(t, 3, src.listCalls())
}

func TestCache_FailedFirstLoadBacksOff(t *testing.T) {
	src := &fakeSource{}
	src.set(errors.New("store down"))
	c, clock := newTestCache(src)

	for range 10 {
		_, _, err := c.Check(context.Background(), netip.MustParseAddr("203.0.113.9"))
		assert.ErrorContains(t, err, "store down")
	}
	assert.Equal(t, 1, src.listCalls(), "no retry against a failing store before the TTL")

	src.set(nil, block("203.0.113.0/24", "spam"))
	clock.Advance(testTTL)
	assert.Equal(t, "spam", checkReason(t, c, "203.0.113.9"))
	assert.Equal(t, 2, src.listCalls())
}

func TestCache_BlocksLapseBetweenLoads(t *testing.T) {
	clock := newTestClock()
	src := &fakeSource{}
	brief := block("203.0.113.0/24", "brief")
	brief.ExpiresAt = clock.Now().Add(10 * time.Second)
	src.set(nil, block("203.0.0.0/16", "broad"), brief)
	c := newCache(src, testTTL, clock.Now)

	assert.Equal(t, "brief", checkReason(t, c, "203.0.113.9"))
	clock.Advance(10 * time.Second)
	assert.Equal(t, "broad", checkReason(t, c, "203.0.113.9"))
	blocks, err := c.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, blocks, 1)
	assert.Equal(t, 1, src.listCalls())
}

func TestCache_ConcurrentFirstCallsShareOneLoad(t *testing.T) {
	src := &fakeSource{}
	src.set(nil, block("203.0.113.0/24", "spam"))
	release := src.hold()
	c, _ := newTestCache(src)

	const callers = 20
	reasons := make(chan string, callers)
	for range callers {
		go func() {
			b, _, err := c.Check(context.Background(), netip.MustParseAddr("203.0.113.9"))
			assert.NoError(t, err)
			reasons <- b.Reason
		}()
	}
	require.Eventually(t, func() bool { return src.listCalls() == 1 }, 5*time.Second, time.Millisecond)
	close(release)
	for range callers {
		assert.Equal(t, "spam", <-reasons)
	}
	assert.Equal(t, 1, src.listCalls())
}

func TestCache_CallerGivingUpDoesNotCancelTheLoad(t *testing.T) {
	src := &fakeSource{}
	src.set(nil, block("203.0.113.0/24", "spam"))
	release := src.hold()
	c, _ := newTestCache(src)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := c.Check(ctx, netip.MustParseAddr("203.0.113.9"))
	assert.ErrorIs(t, err, context.Canceled)

	close(release)
	assert.Equal(t, "spam", checkReason(t, c, "203.0.113.9"))
	assert.Equal(t, 1, src.listCalls())
}

func TestCache_RejectsInvalidAddress(t *testing.T) {
	c, _ := newTestCache(&fakeSource{})
	_, _, err := c.Check(context.Background(), netip.Addr{})
	assert.Error(t, err)
}

func TestCache_OverMemoryListSeesChangesAfterTTL(t *testing.T) {
	ctx := context.Background()
	clock := newTestClock()
	bl := newMemoryList(clock.Now)
	c := newCache(bl, testTTL, clock.Now)
	require.Empty(t, checkReason(t, c, "203.0.113.9"))

	require.NoError(t, bl.Block(ctx, block("203.0.113.0/24", "spam")))
	assert.Empty(t, checkReason(t, c, "203.0.113.9"), "not until the copy is reloaded")
	clock.Advance(testTTL)
	checkReason(t, c, "203.0.113.9")
	waitIdle(t, c)
	assert.Equal(t, "spam", checkReason(t, c, "203.0.113.9"))
}

func TestNewCache_PanicsOnInvalidArguments(t *testing.T) {
	assert.Panics(t, func() { NewCache(nil, time.Minute) })
	assert.Panics(t, func() { NewCache(NewMemoryList(), 0) })
}
