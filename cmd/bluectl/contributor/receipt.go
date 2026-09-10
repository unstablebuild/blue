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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/unstablebuild/blue/cli"
	"github.com/unstablebuild/blue/contributor"
)

func newReceiptCLI(open OpenFunc) cli.CLI {
	return newGroup("receipt", "Import covered-revenue receipts.",
		map[string]cli.CLI{
			actionImport: newReceiptImportCLI(open),
		})
}

type receiptImport struct {
	open OpenFunc
	in   io.Reader
	out  io.Writer
	fs   *cli.FlagSet
}

func newReceiptImportCLI(open OpenFunc) cli.CLI {
	c := &receiptImport{open: open, in: os.Stdin, out: os.Stdout}
	c.fs = cli.NewFlagSet(actionImport)
	return c
}

func (c *receiptImport) Man() cli.Manual {
	return cli.Manual{
		Name: actionImport,
		Summary: "Import receipts from a JSON array, read from a file or " +
			"stdin. Receipts are keyed by their source reference, so " +
			"re-importing a batch is a no-op and the command is safe to " +
			"retry. Each element accepts the fields: source, month, " +
			"grossCents, currency, evidence and adjustments " +
			"[{kind, amountCents, description}].",
		Synopsis: "[<receipts.json>]",
		Options:  *c.fs,
	}
}

func (c *receiptImport) Run(ctx context.Context, args []string) error {
	_, rest, ok, err := cli.ParseUsage(c, c.fs, 0, args)
	if err != nil || !ok {
		return err
	}
	if len(rest) > 1 {
		return cli.ErrInvalidArgs
	}

	source := c.in
	if len(rest) == 1 {
		file, err := os.Open(rest[0])
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		source = file
	}

	var receipts []contributor.Receipt
	if err := json.NewDecoder(source).Decode(&receipts); err != nil {
		return fmt.Errorf("parse receipts: %v", err)
	}
	if len(receipts) == 0 {
		return fmt.Errorf("no receipts to import")
	}
	for i := range receipts {
		receipts[i].Currency = strings.ToLower(receipts[i].Currency)
	}

	ledger, err := c.open(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	imported, duplicates, err := ledger.ImportReceipts(ctx, receipts)
	// Report partial progress: the import is not transactional, and the
	// operator needs to know what landed before the failure.
	_, _ = fmt.Fprintf(c.out, "imported %d receipts, %d already present\n",
		imported, duplicates)
	return err
}
