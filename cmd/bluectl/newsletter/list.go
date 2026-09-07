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

package newsletter

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cli/cliformat"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
)

const (
	defaultListTimeout = 30 * time.Second
	dateTimeFormat     = "2006-01-02T15:04:05.999"

	formatTable = "table"
	formatJSON  = "json"
	formatEmail = "email"
)

// Subscriber mirrors the documents that the website signup endpoint writes to
// the newsletter subscriber collection.
type Subscriber struct {
	// Email is canonical: lowercased and whitespace-trimmed.
	Email string

	// IP is the best-effort address the signup was submitted from.
	IP string

	// SubscribedAt is when the signup was accepted, UTC.
	SubscribedAt time.Time
}

type listCLI struct {
	db     document.Service
	out    io.Writer
	fs     *cli.FlagSet
	format string
	since  string
}

func newListCLI(db document.Service) cli.CLI {
	command := &listCLI{db: db, out: os.Stdout}
	command.fs = cli.NewFlagSet(actionList)
	command.fs.StringVar(&command.format, "F", formatTable,
		"Choose output format. Options: 'table', 'json', 'email' or a Go text/template.")
	command.fs.StringVar(&command.since, "s", "",
		"Only print subscribers that signed up at or after this RFC3339 timestamp.")
	return command
}

func (l *listCLI) Man() cli.Manual {
	return cli.Manual{
		Name: actionList,
		Summary: "Print newsletter subscribers to stdout. Format 'email' prints one " +
			"address per line, to be piped into 'bluectl email send'.",
		Synopsis: "[options]",
		Options:  *l.fs,
	}
}

func (l *listCLI) Run(ctx context.Context, args []string) error {
	_, rest, ok, err := cli.ParseUsage(l, l.fs, 0, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) != 0 {
		return cli.ErrInvalidArgs
	}
	if l.db == nil {
		return errors.New("newsletter subscriber collection is not configured")
	}

	var filters []document.Filter
	if strings.TrimSpace(l.since) != "" {
		since, err := time.Parse(time.RFC3339, l.since)
		if err != nil {
			return err
		}
		filters = append(filters, document.Filter{
			Field: document.Field{
				FieldPath: []string{"SubscribedAt"},
				Value:     since,
			},
			Op: document.OpGreaterThanEqual,
		})
	}

	ctx, cancel := context.WithTimeout(ctx, defaultListTimeout)
	defer cancel()

	documents, err := l.db.List(ctx, filters)
	if err != nil {
		return err
	}
	subscribers := iterator.FromDocumentIterator[Subscriber](documents)
	defer func() { _ = subscribers.Close() }()

	switch strings.ToLower(l.format) {
	case formatJSON:
		return cliformat.JSON[Subscriber]().Format(ctx, l.out, subscribers)
	case formatEmail:
		formatter, err := cliformat.Template[Subscriber]("{{.Email}}")
		if err != nil {
			return err
		}
		return formatter.Format(ctx, l.out, subscribers)
	case formatTable:
		type outSubscriber struct {
			Email        string
			SubscribedAt string
		}
		table := cliformat.Table[outSubscriber]([]string{"Email", "SubscribedAt"})
		return table.Format(ctx, l.out, iterator.Map(subscribers,
			func(subscriber Subscriber) outSubscriber {
				return outSubscriber{
					Email:        subscriber.Email,
					SubscribedAt: subscriber.SubscribedAt.Format(dateTimeFormat),
				}
			}))
	default:
		formatter, err := cliformat.Template[Subscriber](l.format)
		if err != nil {
			return cli.ErrInvalidArgs
		}
		return formatter.Format(ctx, l.out, subscribers)
	}
}
