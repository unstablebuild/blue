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
	"strconv"
	"strings"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/contributor"
	"github.com/unstablebuild/blue/iterator"
)

func newAwardCLI(open OpenFunc) cli.CLI {
	return newGroup("award", "Propose, review and list credit awards.",
		map[string]cli.CLI{
			actionPropose: newAwardProposeCLI(open),
			actionApprove: newAwardDecideCLI(open, actionApprove, true),
			actionReject:  newAwardDecideCLI(open, actionReject, false),
			actionList:    newAwardListCLI(open),
		})
}

// repeatable collects a flag that may be passed more than once.
type repeatable []string

func (r *repeatable) String() string { return strings.Join(*r, ",") }

func (r *repeatable) Set(value string) error {
	*r = append(*r, value)
	return nil
}

type awardPropose struct {
	open        OpenFunc
	out         io.Writer
	fs          *cli.FlagSet
	activation  string
	evidence    repeatable
	description string
}

func newAwardProposeCLI(open OpenFunc) cli.CLI {
	c := &awardPropose{open: open, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionPropose)
	c.fs.StringVar(&c.activation, "activation", "",
		"First month the credits are active, in YYYY-MM form. Defaults to the current month.")
	c.fs.Var(&c.evidence, "e",
		"Public URL backing the award, e.g. a merged pull request. Repeat for more than one.")
	c.fs.StringVar(&c.description, "m", "",
		"Description of the contribution being awarded.")
	return c
}

func (c *awardPropose) Man() cli.Manual {
	return cli.Manual{
		Name: actionPropose,
		Summary: "Propose a credit award. Recipients are named by GitHub " +
			"handle and must already be enrolled. The award is not active " +
			"until it is approved.",
		Synopsis: "[options] <handle>=<credits>...",
		Options:  *c.fs,
	}
}

// parseSplits parses "<handle>=<credits>" arguments.
func parseSplits(args []string) ([]contributor.HandleSplit, error) {
	splits := make([]contributor.HandleSplit, 0, len(args))
	for _, arg := range args {
		handle, amount, ok := strings.Cut(arg, "=")
		if !ok {
			return nil, fmt.Errorf(
				"invalid split %q: expected format is '<handle>=<credits>'",
				arg)
		}
		credits, err := strconv.ParseInt(amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid credits in %q: %v", arg, err)
		}
		splits = append(splits, contributor.HandleSplit{
			GitHubHandle: handle, Credits: credits,
		})
	}
	return splits, nil
}

func (c *awardPropose) Run(ctx context.Context, args []string) error {
	first, rest, ok, err := cli.ParseUsage(c, c.fs, 1, args)
	if err != nil || !ok {
		return err
	}
	positional := make([]string, 0, len(first)+len(rest))
	positional = append(positional, first...)
	positional = append(positional, rest...)
	splits, err := parseSplits(positional)
	if err != nil {
		return err
	}

	activation := contributor.MonthOf(time.Now())
	if c.activation != "" {
		if activation, err = contributor.ParseMonth(c.activation); err != nil {
			return err
		}
	}

	ledger, err := c.open(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	id, err := ledger.ProposeAward(ctx, contributor.ProposeAwardRequest{
		Splits:      splits,
		Activation:  activation,
		Evidence:    c.evidence,
		Description: c.description,
		ProposedBy:  ledger.Operator,
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.out, "proposed award %s activating %s\n",
		id, activation)
	return nil
}

type awardDecide struct {
	open    OpenFunc
	out     io.Writer
	fs      *cli.FlagSet
	action  string
	approve bool
	note    string
}

func newAwardDecideCLI(open OpenFunc, action string, approve bool) cli.CLI {
	c := &awardDecide{
		open: open, out: os.Stdout, action: action, approve: approve,
	}
	c.fs = cli.NewFlagSet(action)
	c.fs.StringVar(&c.note, "m", "", "Note explaining the decision.")
	return c
}

func (c *awardDecide) Man() cli.Manual {
	return cli.Manual{
		Name: c.action,
		Summary: fmt.Sprintf(
			"%s a proposed award. Decisions are final: an award can only "+
				"be decided once.",
			strings.ToUpper(c.action[:1])+c.action[1:]),
		Synopsis: "[options] <award-id>",
		Options:  *c.fs,
	}
}

func (c *awardDecide) Run(ctx context.Context, args []string) error {
	parsed, rest, ok, err := cli.ParseUsage(c, c.fs, 1, args)
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

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	award, err := ledger.DecideAward(ctx, parsed[0], c.approve,
		ledger.Operator, c.note)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.out, "award %s is now %s\n", award.ID, award.Status)
	return nil
}

type awardList struct {
	open   OpenFunc
	out    io.Writer
	fs     *cli.FlagSet
	format string
}

func newAwardListCLI(open OpenFunc) cli.CLI {
	c := &awardList{open: open, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionList)
	c.fs.StringVar(&c.format, "F", formatTable, formatUsage)
	return c
}

func (c *awardList) Man() cli.Manual {
	return cli.Manual{
		Name: actionList,
		Summary: "Print awards to stdout, optionally filtered by status " +
			"('proposed', 'approved' or 'rejected').",
		Synopsis: "[options] [<status>]",
		Options:  *c.fs,
	}
}

func (c *awardList) Run(ctx context.Context, args []string) error {
	_, rest, ok, err := cli.ParseUsage(c, c.fs, 0, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) > 1 {
		return cli.ErrInvalidArgs
	}
	var status contributor.AwardStatus
	if len(rest) == 1 {
		status = contributor.AwardStatus(rest[0])
	}

	ledger, err := c.open(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	it, err := ledger.Awards.ListAwards(ctx, status)
	if err != nil {
		return err
	}
	awards, err := iterator.ToSlice(ctx, it)
	if err != nil {
		return err
	}

	type row struct {
		ID          string
		Status      string
		Activation  string
		Expiration  string
		Credits     int64
		Recipients  string
		ProposedBy  string
		DecidedBy   string
		Description string
	}
	return render(ctx, c.out, c.format,
		[]string{"ID", "Status", "Activation", "Expiration", "Credits",
			"Recipients", "ProposedBy", "DecidedBy", "Description"},
		awards,
		func(a contributor.Award) row {
			recipients := make([]string, 0, len(a.Splits))
			for _, s := range a.Splits {
				recipients = append(recipients,
					fmt.Sprintf("%s:%d", s.Account, s.Credits))
			}
			return row{
				ID:          a.ID,
				Status:      string(a.Status),
				Activation:  string(a.Activation),
				Expiration:  string(a.Expiration),
				Credits:     a.TotalCredits(),
				Recipients:  strings.Join(recipients, " "),
				ProposedBy:  a.ProposedBy,
				DecidedBy:   a.DecidedBy,
				Description: a.Description,
			}
		})
}
