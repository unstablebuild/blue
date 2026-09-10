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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/contributor"
	"github.com/unstablebuild/blue/iterator"
)

func newProgramCLI(open OpenFunc) cli.CLI {
	return newGroup("program", "Publish and inspect program policy versions.",
		map[string]cli.CLI{
			actionSet:  newProgramSetCLI(open),
			actionShow: newProgramShowCLI(open),
		})
}

type programSet struct {
	open    OpenFunc
	out     io.Writer
	fs      *cli.FlagSet
	program contributor.Program
	month   string
}

func newProgramSetCLI(open OpenFunc) cli.CLI {
	c := &programSet{open: open, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionSet)
	c.fs.StringVar(&c.program.Version, "version", "",
		"Policy revision identifier, e.g. '2026-01'.")
	c.fs.StringVar(&c.program.Currency, "currency", "usd",
		"ISO 4217 currency all receipts, pools and obligations are denominated in.")
	c.fs.IntVar(&c.program.PoolBps, "pool-bps", 0,
		"Share of covered net receipts allocated to the monthly pool, in basis points (2000 = 20%).")
	c.fs.StringVar(&c.program.CoveredReceipts, "covered-receipts", "",
		"Human-readable definition of which receipts the program covers.")
	c.fs.IntVar(&c.program.CreditDurationRounds, "credit-duration-rounds", 0,
		"Number of monthly rounds an awarded credit stays active.")
	c.fs.StringVar(&c.program.PaymentTimetable, "payment-timetable", "",
		"Human-readable description of when entitlements become owed and are paid.")
	c.fs.IntVar(&c.program.PayoutDueDays, "payout-due-days", 0,
		"Days after the end of a round's calendar quarter in which its obligations become due.")
	c.fs.StringVar(&c.month, "effective-from", "",
		"First month this policy applies to, in YYYY-MM form.")
	return c
}

func (c *programSet) Man() cli.Manual {
	return cli.Manual{
		Name: actionSet,
		Summary: "Publish a program policy version. Policies are immutable " +
			"once published: a change is a new version with a later " +
			"effective month.",
		Synopsis: "[options]",
		Options:  *c.fs,
	}
}

func (c *programSet) Run(ctx context.Context, args []string) error {
	_, rest, ok, err := cli.ParseUsage(c, c.fs, 0, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) != 0 {
		return cli.ErrInvalidArgs
	}

	ledger, err := c.open(ctx)
	if err != nil {
		return err
	}

	program := c.program
	program.Currency = strings.ToLower(program.Currency)
	program.EffectiveFrom = contributor.Month(c.month)

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	if err := ledger.SetProgram(ctx, program); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.out, "published program %s effective from %s\n",
		program.Version, program.EffectiveFrom)
	return nil
}

type programShow struct {
	open   OpenFunc
	out    io.Writer
	fs     *cli.FlagSet
	format string
	all    bool
}

func newProgramShowCLI(open OpenFunc) cli.CLI {
	c := &programShow{open: open, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionShow)
	c.fs.StringVar(&c.format, "F", formatTable, formatUsage)
	c.fs.BoolVar(&c.all, "a", false,
		"Print every published policy version instead of the effective one.")
	return c
}

func (c *programShow) Man() cli.Manual {
	return cli.Manual{
		Name: actionShow,
		Summary: "Print the policy effective in a month, defaulting to the " +
			"current month.",
		Synopsis: "[options] [<month>]",
		Options:  *c.fs,
	}
}

func (c *programShow) Run(ctx context.Context, args []string) error {
	_, rest, ok, err := cli.ParseUsage(c, c.fs, 0, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) > 1 {
		return cli.ErrInvalidArgs
	}

	ledger, err := c.open(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	var programs []contributor.Program
	if c.all {
		it, err := ledger.Programs.ListPrograms(ctx)
		if err != nil {
			return err
		}
		if programs, err = iterator.ToSlice(ctx, it); err != nil {
			return err
		}
	} else {
		month, err := parseMonthArg(rest)
		if err != nil {
			return err
		}
		program, err := ledger.Programs.ProgramFor(ctx, month)
		if err != nil {
			return fmt.Errorf("program for %s: %w", month, err)
		}
		programs = []contributor.Program{program}
	}

	type row struct {
		Version         string
		EffectiveFrom   string
		Currency        string
		PoolBps         int
		CreditDuration  int
		PayoutDueDays   int
		CalculationVer  int
		CoveredReceipts string
	}
	return render(ctx, c.out, c.format,
		[]string{"Version", "EffectiveFrom", "Currency", "PoolBps",
			"CreditDuration", "PayoutDueDays", "CalculationVer",
			"CoveredReceipts"},
		programs,
		func(p contributor.Program) row {
			return row{
				Version:         p.Version,
				EffectiveFrom:   string(p.EffectiveFrom),
				Currency:        p.Currency,
				PoolBps:         p.PoolBps,
				CreditDuration:  p.CreditDurationRounds,
				PayoutDueDays:   p.PayoutDueDays,
				CalculationVer:  p.CalculationVersion,
				CoveredReceipts: p.CoveredReceipts,
			}
		})
}
