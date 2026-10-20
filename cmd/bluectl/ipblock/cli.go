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

// Package ipblock implements bluectl's ipblock commands, which manage
// the blocklist of IP addresses that services sharing it refuse.
package ipblock

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/ipblock"
)

const (
	actionAdd    = "add"
	actionRemove = "remove"
	actionList   = "list"

	defaultTimeout = 30 * time.Second
)

var errNotConfigured = errors.New("the ipblock collection is not configured")

type ipblockCLI struct {
	cmds map[string]cli.CLI
	fs   *cli.FlagSet
}

// NewCLI returns the bluectl ipblock command. blocklist may be nil when
// the command is created only to render help. operator is recorded as
// who blocked the addresses it adds.
func NewCLI(blocklist ipblock.List, operator string) cli.CLI {
	return &ipblockCLI{
		cmds: map[string]cli.CLI{
			actionAdd:    newAddCLI(blocklist, operator),
			actionRemove: newRemoveCLI(blocklist),
			actionList:   newListCLI(blocklist),
		},
		fs: cli.NewFlagSet("ipblock"),
	}
}

func (c *ipblockCLI) Man() cli.Manual {
	commands := make([]cli.Manual, 0, len(c.cmds))
	for _, command := range c.cmds {
		commands = append(commands, command.Man())
	}
	return cli.Manual{
		Name: "ipblock",
		Summary: "Block IP addresses and prefixes for every service that " +
			"checks the blocklist.",
		Synopsis: "<cmd>",
		Commands: commands,
		Options:  *c.fs,
	}
}

func (c *ipblockCLI) Run(ctx context.Context, args []string) error {
	return cli.ParseAndRunCommand(ctx, c, c.fs, c.cmds, args)
}

// parsePrefixes parses every argument before anything is changed, so a
// typo in the last one does not leave the others half applied.
func parsePrefixes(args []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(args))
	for _, arg := range args {
		p, err := parsePrefix(arg)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, p)
	}
	return prefixes, nil
}

// parsePrefix parses an address, which stands for the prefix of that
// address alone, or a prefix. It returns the prefix as the blocklist
// stores it, so what is printed is what is blocked.
func parsePrefix(s string) (netip.Prefix, error) {
	var p netip.Prefix
	if strings.Contains(s, "/") {
		var err error
		if p, err = netip.ParsePrefix(s); err != nil {
			return netip.Prefix{}, err
		}
	} else {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("%q is neither an address nor a prefix", s)
		}
		addr = addr.WithZone("")
		p = netip.PrefixFrom(addr, addr.BitLen())
	}
	if addr := p.Addr(); addr.Is4In6() && p.Bits() >= 96 {
		p = netip.PrefixFrom(addr.Unmap(), p.Bits()-96)
	}
	return p.Masked(), nil
}
