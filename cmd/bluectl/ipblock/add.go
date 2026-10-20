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
	"fmt"
	"io"
	"net/netip"
	"os"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/ipblock"
)

const (
	// broadestIPv4 and broadestIPv6 are the broadest prefixes add blocks
	// without -f. Anything broader is more likely a typo than a target.
	broadestIPv4 = 16
	broadestIPv6 = 32
)

type addCLI struct {
	blocklist ipblock.List
	operator  string
	now       func() time.Time
	out       io.Writer
	fs        *cli.FlagSet
	ttl       time.Duration
	reason    string
	force     bool
}

func newAddCLI(blocklist ipblock.List, operator string) cli.CLI {
	c := &addCLI{blocklist: blocklist, operator: operator, now: time.Now, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionAdd)
	c.fs.DurationVar(&c.ttl, "t", 0,
		"Lift the blocks after this long, e.g. 24h. Zero keeps them until removed.")
	c.fs.StringVar(&c.reason, "r", "", "Record why the addresses are blocked.")
	c.fs.BoolVar(&c.force, "f", false, fmt.Sprintf(
		"Allow prefixes broader than /%d for IPv4 or /%d for IPv6.", broadestIPv4, broadestIPv6))
	return c
}

func (c *addCLI) Man() cli.Manual {
	return cli.Manual{
		Name: actionAdd,
		Summary: "Block addresses or prefixes. Blocking a prefix that is " +
			"already blocked replaces its block. Services pick blocks up " +
			"within their cache TTL.",
		Synopsis: "[options] <address|prefix>...",
		Options:  *c.fs,
	}
}

func (c *addCLI) Run(ctx context.Context, args []string) error {
	parsed, rest, ok, err := cli.ParseUsage(c, c.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	if c.ttl < 0 {
		return fmt.Errorf("-t must not be negative, got %s", c.ttl)
	}
	prefixes, err := parsePrefixes(append(parsed, rest...))
	if err != nil {
		return err
	}
	if !c.force {
		for _, p := range prefixes {
			if tooBroad(p) {
				return fmt.Errorf("%s blocks more than a /%d (IPv4) or /%d (IPv6); pass -f if that is intended",
					p, broadestIPv4, broadestIPv6)
			}
		}
	}
	if c.blocklist == nil {
		return errNotConfigured
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	now := c.now()
	var expiresAt time.Time
	if c.ttl > 0 {
		expiresAt = now.Add(c.ttl)
	}
	for _, p := range prefixes {
		err := c.blocklist.Block(ctx, ipblock.Block{
			Prefix:    p,
			Reason:    c.reason,
			BlockedBy: c.operator,
			BlockedAt: now,
			ExpiresAt: expiresAt,
		})
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(c.out, "blocked %s %s\n", p, until(expiresAt))
	}
	return nil
}

// until describes when a block expiring at t lapses.
func until(t time.Time) string {
	if t.IsZero() {
		return "until removed"
	}
	return "until " + t.UTC().Format(time.RFC3339)
}

// tooBroad reports whether p is broader than add blocks without -f.
func tooBroad(p netip.Prefix) bool {
	if p.Addr().Is4() {
		return p.Bits() < broadestIPv4
	}
	return p.Bits() < broadestIPv6
}
