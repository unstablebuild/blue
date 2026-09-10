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

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cli/cliformat"
	"github.com/unstablebuild/blue/contributor"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
)

func newRoundCLI(open OpenFunc) cli.CLI {
	return newGroup("round", "Close and inspect monthly allocation rounds.",
		map[string]cli.CLI{
			actionClose: newRoundCloseCLI(open),
			actionShow:  newRoundShowCLI(open),
		})
}

type roundClose struct {
	open OpenFunc
	out  io.Writer
	fs   *cli.FlagSet
}

func newRoundCloseCLI(open OpenFunc) cli.CLI {
	c := &roundClose{open: open, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionClose)
	return c
}

func (c *roundClose) Man() cli.Manual {
	return cli.Manual{
		Name: actionClose,
		Summary: "Freeze a month: compute its allocation from the effective " +
			"policy, receipts and approved awards, then persist the round " +
			"and its obligations. A closed round is immutable.",
		Synopsis: "<month>",
		Options:  *c.fs,
	}
}

func (c *roundClose) Run(ctx context.Context, args []string) error {
	parsed, rest, ok, err := cli.ParseUsage(c, c.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) != 0 {
		return cli.ErrInvalidArgs
	}
	month, err := contributor.ParseMonth(parsed[0])
	if err != nil {
		return err
	}

	ledger, err := c.open(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	round, err := ledger.CloseRound(ctx, month, time.Now().UTC(),
		ledger.Operator)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.out,
		"closed round %s: pool %d cents over %d credits across %d entitlements\n",
		round.Month, round.PoolCents, round.TotalCredits,
		len(round.Entitlements))
	return nil
}

type roundShow struct {
	open   OpenFunc
	out    io.Writer
	fs     *cli.FlagSet
	format string
}

func newRoundShowCLI(open OpenFunc) cli.CLI {
	c := &roundShow{open: open, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionShow)
	c.fs.StringVar(&c.format, "F", formatTable, formatUsage)
	return c
}

func (c *roundShow) Man() cli.Manual {
	return cli.Manual{
		Name: actionShow,
		Summary: "Print a round's per-participant entitlements, defaulting " +
			"to the current month. Months that are not closed yet are " +
			"estimated from current data.",
		Synopsis: "[options] [<month>]",
		Options:  *c.fs,
	}
}

func (c *roundShow) Run(ctx context.Context, args []string) error {
	_, rest, ok, err := cli.ParseUsage(c, c.fs, 0, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) > 1 {
		return cli.ErrInvalidArgs
	}
	month, err := parseMonthArg(rest)
	if err != nil {
		return err
	}

	ledger, err := c.open(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	round, err := ledger.Rounds.GetRound(ctx, month)
	if errors.Is(err, document.ErrNotFound) {
		if round, _, err = ledger.Estimate(ctx, month); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// A round is one record with a nested table of entitlements: 'table'
	// summarizes it and lists the entitlements, while 'json' and
	// templates emit the whole record so it stays pipeable.
	switch strings.ToLower(c.format) {
	case formatTable:
		_, _ = fmt.Fprintf(c.out,
			"round %s (%s): net receipts %d, carryforward %d, pool %d %s over %d credits\n",
			round.Month, roundState(round), round.NetReceiptsCents,
			round.CarryforwardCents, round.PoolCents, round.Currency,
			round.TotalCredits)

		type row struct {
			Account     string
			Credits     int64
			AmountCents int64
		}
		return cliformat.Table[row](
			[]string{"Account", "Credits", "AmountCents"},
		).Format(ctx, c.out, iterator.Map(
			iterator.FromSlice(round.Entitlements),
			func(e contributor.Entitlement) row {
				return row{
					Account:     e.Account,
					Credits:     e.Credits,
					AmountCents: e.AmountCents,
				}
			}))
	case formatJSON:
		return cliformat.JSON[contributor.Round]().Format(ctx, c.out,
			iterator.FromSlice([]contributor.Round{round}))
	default:
		formatter, err := cliformat.Template[contributor.Round](c.format)
		if err != nil {
			return cli.ErrInvalidArgs
		}
		return formatter.Format(ctx, c.out,
			iterator.FromSlice([]contributor.Round{round}))
	}
}

// roundState reports whether the round was read from the ledger or
// computed on the fly, which is the difference between a final and a
// provisional number.
func roundState(r contributor.Round) string {
	if r.ClosedAt.IsZero() {
		return "estimated"
	}
	return fmt.Sprintf("closed %s by %s", formatTime(r.ClosedAt), r.ClosedBy)
}
