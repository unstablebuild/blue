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
	"fmt"
	"io"
	"os"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/ipblock"
)

type removeCLI struct {
	blocklist ipblock.List
	out       io.Writer
	fs        *cli.FlagSet
}

func newRemoveCLI(blocklist ipblock.List) cli.CLI {
	c := &removeCLI{blocklist: blocklist, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionRemove)
	return c
}

func (c *removeCLI) Man() cli.Manual {
	return cli.Manual{
		Name: actionRemove,
		Summary: "Lift the blocks on exactly these addresses or prefixes; " +
			"blocks on prefixes containing them stay. Services pick the " +
			"change up within their cache TTL.",
		Synopsis: "<address|prefix>...",
		Options:  *c.fs,
	}
}

func (c *removeCLI) Run(ctx context.Context, args []string) error {
	parsed, rest, ok, err := cli.ParseUsage(c, c.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	prefixes, err := parsePrefixes(append(parsed, rest...))
	if err != nil {
		return err
	}
	if c.blocklist == nil {
		return errNotConfigured
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	// Lift every block that is there before reporting the ones that
	// were not.
	var errs []error
	for _, p := range prefixes {
		err := c.blocklist.Unblock(ctx, p)
		switch {
		case errors.Is(err, ipblock.ErrNotBlocked):
			errs = append(errs, fmt.Errorf("%s is not blocked", p))
		case err != nil:
			return errors.Join(append(errs, err)...)
		default:
			_, _ = fmt.Fprintf(c.out, "unblocked %s\n", p)
		}
	}
	return errors.Join(errs...)
}
