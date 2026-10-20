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
	"io"
	"os"
	"strings"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cli/cliformat"
	"github.com/unstablebuild/blue/ipblock"
	"github.com/unstablebuild/blue/iterator"
)

const (
	formatTable = "table"
	formatJSON  = "json"
)

// listedBlock is a block as list prints it as JSON or through a
// template.
type listedBlock struct {
	Prefix    string
	Reason    string
	BlockedBy string
	BlockedAt time.Time
	// ExpiresAt is nil for blocks that are kept until removed.
	ExpiresAt *time.Time `json:",omitempty"`
}

type listCLI struct {
	blocklist ipblock.List
	out       io.Writer
	fs        *cli.FlagSet
	format    string
}

func newListCLI(blocklist ipblock.List) cli.CLI {
	c := &listCLI{blocklist: blocklist, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionList)
	c.fs.StringVar(&c.format, "F", formatTable,
		"Choose output format. Options: 'table', 'json' or a Go text/template.")
	return c
}

func (c *listCLI) Man() cli.Manual {
	return cli.Manual{
		Name:     actionList,
		Summary:  "Print the blocks in force, ordered by prefix.",
		Synopsis: "[options]",
		Options:  *c.fs,
	}
}

func (c *listCLI) Run(ctx context.Context, args []string) error {
	_, rest, ok, err := cli.ParseUsage(c, c.fs, 0, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) != 0 {
		return cli.ErrInvalidArgs
	}
	if c.blocklist == nil {
		return errNotConfigured
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	blocks, err := c.blocklist.List(ctx)
	if err != nil {
		return err
	}
	listed := make([]listedBlock, 0, len(blocks))
	for _, b := range blocks {
		l := listedBlock{
			Prefix:    b.Prefix.String(),
			Reason:    b.Reason,
			BlockedBy: b.BlockedBy,
			BlockedAt: b.BlockedAt.UTC(),
		}
		if !b.ExpiresAt.IsZero() {
			expiresAt := b.ExpiresAt.UTC()
			l.ExpiresAt = &expiresAt
		}
		listed = append(listed, l)
	}

	switch strings.ToLower(c.format) {
	case formatJSON:
		return cliformat.JSON[listedBlock]().Format(ctx, c.out, iterator.FromSlice(listed))
	case formatTable:
		type row struct {
			Prefix    string
			Expires   string
			BlockedBy string
			BlockedAt string
			Reason    string
		}
		table := cliformat.Table[row]([]string{"Prefix", "Expires", "BlockedBy", "BlockedAt", "Reason"})
		return table.Format(ctx, c.out, iterator.Map(iterator.FromSlice(listed),
			func(l listedBlock) row {
				expires := "never"
				if l.ExpiresAt != nil {
					expires = l.ExpiresAt.Format(time.RFC3339)
				}
				return row{
					Prefix:    l.Prefix,
					Expires:   expires,
					BlockedBy: l.BlockedBy,
					BlockedAt: l.BlockedAt.Format(time.RFC3339),
					Reason:    l.Reason,
				}
			}))
	default:
		formatter, err := cliformat.Template[listedBlock](c.format)
		if err != nil {
			return cli.ErrInvalidArgs
		}
		return formatter.Format(ctx, c.out, iterator.FromSlice(listed))
	}
}
