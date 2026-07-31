// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
