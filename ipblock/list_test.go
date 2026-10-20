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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testClock is a clock tests move by hand.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock() *testClock {
	return &testClock{now: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// listFactory returns an empty List that tells time by now.
type listFactory func(t *testing.T, now func() time.Time) List

// testList is the contract every List implementation keeps.
func testList(t *testing.T, newList listFactory) {
	ctx := context.Background()
	setup := func(t *testing.T) (List, *testClock) {
		clock := newTestClock()
		return newList(t, clock.Now), clock
	}
	prefix := netip.MustParsePrefix
	addr := netip.MustParseAddr

	t.Run("matches addresses against the narrowest block", func(t *testing.T) {
		bl, clock := setup(t)
		for _, b := range []Block{
			{Prefix: prefix("203.0.113.7/32"), Reason: "single", BlockedBy: "ops"},
			{Prefix: prefix("198.51.100.0/24"), Reason: "range"},
			{Prefix: prefix("198.51.100.128/25"), Reason: "narrow"},
			{Prefix: prefix("2001:db8:1:2::/64"), Reason: "v6"},
			{Prefix: prefix("::ffff:192.0.2.1/128"), Reason: "mapped"},
			{Prefix: prefix("100.64.5.9/16"), Reason: "masked"},
		} {
			require.NoError(t, bl.Block(ctx, b))
		}

		cases := []struct {
			addr       string
			wantReason string
		}{
			{addr: "203.0.113.7", wantReason: "single"},
			{addr: "203.0.113.8"},
			{addr: "198.51.100.5", wantReason: "range"},
			{addr: "198.51.100.200", wantReason: "narrow"},
			{addr: "2001:db8:1:2:aaaa::1", wantReason: "v6"},
			{addr: "2001:db8:1:2::1%eth0", wantReason: "v6"},
			{addr: "2001:db8:1:3::1"},
			{addr: "::ffff:203.0.113.7", wantReason: "single"},
			{addr: "192.0.2.1", wantReason: "mapped"},
			{addr: "100.64.200.1", wantReason: "masked"},
		}
		for _, tc := range cases {
			t.Run(tc.addr, func(t *testing.T) {
				b, ok, err := bl.Check(ctx, addr(tc.addr))
				require.NoError(t, err)
				assert.Equal(t, tc.wantReason != "", ok)
				assert.Equal(t, tc.wantReason, b.Reason)
			})
		}

		blocks, err := bl.List(ctx)
		require.NoError(t, err)
		var got []string
		for _, b := range blocks {
			got = append(got, b.Prefix.String())
			assert.True(t, clock.Now().Equal(b.BlockedAt), "BlockedAt defaults to now")
		}
		assert.Equal(t, []string{
			"100.64.0.0/16", "192.0.2.1/32", "198.51.100.0/24",
			"198.51.100.128/25", "203.0.113.7/32", "2001:db8:1:2::/64",
		}, got, "canonical prefixes, ordered")
		assert.Equal(t, "ops", blocks[4].BlockedBy)
	})

	t.Run("lapsed blocks are neither matched nor listed", func(t *testing.T) {
		bl, clock := setup(t)
		require.NoError(t, bl.Block(ctx, Block{Prefix: prefix("10.0.0.0/8"), Reason: "broad"}))
		require.NoError(t, bl.Block(ctx, Block{
			Prefix: prefix("10.1.2.3/32"), Reason: "brief", ExpiresAt: clock.Now().Add(time.Hour),
		}))

		b, ok, err := bl.Check(ctx, addr("10.1.2.3"))
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "brief", b.Reason)
		assert.True(t, clock.Now().Add(time.Hour).Equal(b.ExpiresAt))

		clock.Advance(time.Hour)
		b, ok, err = bl.Check(ctx, addr("10.1.2.3"))
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "broad", b.Reason, "the broader block shows through")
		blocks, err := bl.List(ctx)
		require.NoError(t, err)
		require.Len(t, blocks, 1)
		assert.Equal(t, "broad", blocks[0].Reason)
		assert.True(t, blocks[0].ExpiresAt.IsZero())
	})

	t.Run("blocking again replaces the block", func(t *testing.T) {
		bl, clock := setup(t)
		require.NoError(t, bl.Block(ctx, Block{
			Prefix: prefix("203.0.113.0/24"), Reason: "first", ExpiresAt: clock.Now().Add(time.Hour),
		}))
		require.NoError(t, bl.Block(ctx, Block{Prefix: prefix("203.0.113.0/24"), Reason: "second"}))

		clock.Advance(2 * time.Hour)
		blocks, err := bl.List(ctx)
		require.NoError(t, err)
		require.Len(t, blocks, 1)
		assert.Equal(t, "second", blocks[0].Reason)
		assert.True(t, blocks[0].ExpiresAt.IsZero(), "the replacement never lapses")
	})

	t.Run("unblock lifts exactly the prefix", func(t *testing.T) {
		bl, _ := setup(t)
		require.NoError(t, bl.Block(ctx, Block{Prefix: prefix("10.0.0.0/8"), Reason: "broad"}))
		require.NoError(t, bl.Block(ctx, Block{Prefix: prefix("10.1.0.0/16"), Reason: "narrow"}))

		require.NoError(t, bl.Unblock(ctx, prefix("10.1.0.0/16")))
		b, ok, err := bl.Check(ctx, addr("10.1.2.3"))
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "broad", b.Reason)

		assert.ErrorIs(t, bl.Unblock(ctx, prefix("10.1.0.0/16")), ErrNotBlocked, "already lifted")
		assert.ErrorIs(t, bl.Unblock(ctx, prefix("10.0.0.0/16")), ErrNotBlocked, "inside a block, not one")
		require.NoError(t, bl.Unblock(ctx, prefix("10.9.9.9/8")), "matched once masked")

		_, ok, err = bl.Check(ctx, addr("10.1.2.3"))
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("unblocking a lapsed block reports it was not blocked", func(t *testing.T) {
		bl, clock := setup(t)
		require.NoError(t, bl.Block(ctx, Block{
			Prefix: prefix("203.0.113.7/32"), ExpiresAt: clock.Now().Add(time.Minute),
		}))
		clock.Advance(time.Minute)

		assert.ErrorIs(t, bl.Unblock(ctx, prefix("203.0.113.7/32")), ErrNotBlocked)
		assert.ErrorIs(t, bl.Unblock(ctx, prefix("203.0.113.7/32")), ErrNotBlocked)
	})

	t.Run("rejects invalid input", func(t *testing.T) {
		bl, clock := setup(t)
		cases := []struct {
			name string
			call func() error
		}{
			{name: "block without a prefix", call: func() error { return bl.Block(ctx, Block{Reason: "x"}) }},
			{name: "block expiring now", call: func() error {
				return bl.Block(ctx, Block{Prefix: prefix("203.0.113.7/32"), ExpiresAt: clock.Now()})
			}},
			{name: "block that already expired", call: func() error {
				return bl.Block(ctx, Block{Prefix: prefix("203.0.113.7/32"), ExpiresAt: clock.Now().Add(-time.Hour)})
			}},
			{name: "unblock without a prefix", call: func() error { return bl.Unblock(ctx, netip.Prefix{}) }},
			{name: "check without an address", call: func() error {
				_, _, err := bl.Check(ctx, netip.Addr{})
				return err
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.call()
				require.Error(t, err)
				assert.NotErrorIs(t, err, ErrNotBlocked)
			})
		}
		blocks, err := bl.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, blocks)
	})
}

func TestMemoryList(t *testing.T) {
	testList(t, func(_ *testing.T, now func() time.Time) List {
		return newMemoryList(now)
	})
}

func TestCanonical(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{in: "203.0.113.7/32", want: "203.0.113.7/32"},
		{in: "203.0.113.7/24", want: "203.0.113.0/24"},
		{in: "::ffff:203.0.113.7/120", want: "203.0.113.0/24"},
		{in: "::ffff:0:0/80", want: "::/80"},
		{in: "2001:db8::1/64", want: "2001:db8::/64"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := canonical(netip.MustParsePrefix(tc.in))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.String())
		})
	}

	_, err := canonical(netip.Prefix{})
	assert.Error(t, err)
}
