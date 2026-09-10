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
	"strings"
	"time"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/cli/cliformat"
	"github.com/unstablebuild/blue/iterator"
)

const (
	formatTable = "table"
	formatJSON  = "json"

	formatUsage = "Choose output format. Options: 'table', 'json' or a " +
		"Go text/template."

	dateTimeFormat = "2006-01-02T15:04:05.999"
)

// render writes els in the format selected by -F. The table format
// projects each element through row so it can flatten nested fields into
// columns; 'json' and templates always see the domain type, so a piped
// export stays faithful to the ledger.
func render[T, R any](
	ctx context.Context,
	out io.Writer,
	format string,
	columns []string,
	els []T,
	row func(T) R,
) error {
	switch strings.ToLower(format) {
	case formatTable:
		return cliformat.Table[R](columns).Format(ctx, out,
			iterator.Map(iterator.FromSlice(els), row))
	case formatJSON:
		return cliformat.JSON[T]().Format(ctx, out, iterator.FromSlice(els))
	default:
		formatter, err := cliformat.Template[T](format)
		if err != nil {
			return cli.ErrInvalidArgs
		}
		return formatter.Format(ctx, out, iterator.FromSlice(els))
	}
}

// formatTime renders t for table output, leaving zero times blank rather
// than printing a meaningless epoch.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(dateTimeFormat)
}
