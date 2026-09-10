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
	"io"
	"os"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/contributor"
	"github.com/unstablebuild/blue/iterator"
)

func newParticipantCLI(open OpenFunc) cli.CLI {
	return newGroup("participant", "Inspect enrolled contributors.",
		map[string]cli.CLI{
			actionList: newParticipantListCLI(open),
		})
}

type participantList struct {
	open   OpenFunc
	out    io.Writer
	fs     *cli.FlagSet
	format string
}

func newParticipantListCLI(open OpenFunc) cli.CLI {
	c := &participantList{open: open, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionList)
	c.fs.StringVar(&c.format, "F", formatTable, formatUsage)
	return c
}

func (c *participantList) Man() cli.Manual {
	return cli.Manual{
		Name: actionList,
		Summary: "Print enrolled contributors to stdout, including the " +
			"GitHub handles that 'award propose' accepts.",
		Synopsis: "[options]",
		Options:  *c.fs,
	}
}

func (c *participantList) Run(ctx context.Context, args []string) error {
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

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	it, err := ledger.Participants.ListParticipants(ctx)
	if err != nil {
		return err
	}
	participants, err := iterator.ToSlice(ctx, it)
	if err != nil {
		return err
	}

	type row struct {
		GitHubHandle string
		Account      string
		Status       string
		PayoutReady  bool
		Agreement    string
		EnrolledAt   string
	}
	return render(ctx, c.out, c.format,
		[]string{"GitHubHandle", "Account", "Status", "PayoutReady",
			"Agreement", "EnrolledAt"},
		participants,
		func(p contributor.Participant) row {
			return row{
				GitHubHandle: p.GitHubHandle,
				Account:      p.Account,
				Status:       string(p.Status),
				PayoutReady:  p.PayoutReady,
				Agreement:    p.AgreementVersion,
				EnrolledAt:   formatTime(p.AgreementAcceptedAt),
			}
		})
}
