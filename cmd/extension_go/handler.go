// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/unstablebuild/blue/ide/idelsp/lspcmd"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

const cmdName = "go"

func newGoHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, notify browserapi.Notifications,
) (textapi.CommandManual, textapi.CommandHandler, error) {
	sel := lspcmd.NewSelectionTracker()
	evs := []textapi.EventType{textapi.EventTypeSelection, textapi.EventTypeCursor}
	if err := editor.SubscribeEvents(evs, sel); err != nil {
		return textapi.CommandManual{}, nil, fmt.Errorf("subscribe selection events: %w", err)
	}

	handlers := map[string]textapi.CommandHandler{
		// Source actions.
		"organize-imports": codeActionHandler(lsp, editor, notify, wm, sel, "source.organizeImports",
			"Imports are already organized"),
		"fix-all": codeActionHandler(lsp, editor, notify, wm, sel, "source.fixAll",
			"No automatic fixes available"),
		"add-test": codeActionHandler(lsp, editor, notify, wm, sel, "source.addTest",
			"Place cursor inside a function to generate a test"),
		"assembly": codeActionHandler(lsp, editor, notify, wm, sel, "source.assembly",
			"Place cursor inside a function to view assembly. Generic functions and init() are not supported"),
		"doc": codeActionHandler(lsp, editor, notify, wm, sel, "source.doc",
			"No documentation available for this package"),
		"free-symbols": codeActionHandler(lsp, editor, notify, wm, sel, "source.freesymbols",
			"Select a block of code to analyze its free symbols"),

		// Refactoring actions.
		"fill-struct": codeActionHandler(lsp, editor, notify, wm, sel, "refactor.rewrite.fillStruct",
			"Place cursor on a struct literal (e.g. Type{}) to fill fields"),
		"fill-switch": codeActionHandler(lsp, editor, notify, wm, sel, "refactor.rewrite.fillSwitch",
			"Place cursor on a switch statement to add missing cases"),
		"add-tags": codeActionHandler(lsp, editor, notify, wm, sel, "refactor.rewrite.addTags",
			"Place cursor on a struct field to add tags"),
		"remove-tags": codeActionHandler(lsp, editor, notify, wm, sel, "refactor.rewrite.removeTags",
			"Place cursor on a struct field with existing tags"),
		"extract-function": codeActionHandler(lsp, editor, notify, wm, sel, "refactor.extract.function",
			"Select one or more complete statements to extract"),
		"extract-variable": codeActionHandler(lsp, editor, notify, wm, sel, "refactor.extract.variable",
			"Select an expression to extract as a variable"),
		"inline-call": codeActionHandler(lsp, editor, notify, wm, sel, "refactor.inline.call",
			"Place cursor on a function call to inline it"),
		"invert-if": codeActionHandler(lsp, editor, notify, wm, sel, "refactor.rewrite.invertIf",
			"Place cursor on an if-else statement to invert its condition"),

		// Code lens commands.
		"test":     codeLensHandler(lsp, notify, "gopls.run_tests"),
		"generate": codeLensHandler(lsp, notify, "gopls.generate"),

		// Module management.
		"tidy":      modCommandHandler(lsp, notify, "gopls.tidy"),
		"vendor":    modCommandHandler(lsp, notify, "gopls.vendor"),
		"vulncheck": vulncheckHandler(lsp, notify),

		// Imports.
		"add-import": addImportHandler(lsp, notify),
	}

	manual := textapi.CommandManual{
		Name:     cmdName,
		Summary:  "Go language commands powered by gopls",
		Synopsis: "<subcommand> [<args>...]",
		Commands: []textapi.CommandManual{
			{Name: "organize-imports", Summary: "Organize imports by removing unused, adding missing, and sorting into conventional order"},
			{Name: "fix-all", Summary: "Apply all unambiguously safe fixes to code issues"},
			{Name: "add-test", Summary: "Generate a table-driven test for the function or method at the cursor"},
			{Name: "assembly", Summary: "Show the assembly produced by the compiler for the function at the cursor"},
			{Name: "doc", Summary: "Browse documentation for the current Go package"},
			{Name: "free-symbols", Summary: "Analyze the selected code and report symbols referenced within it but defined outside it"},
			{Name: "fill-struct", Summary: "Fill each missing field in a struct literal with a zero value or matching variable"},
			{Name: "fill-switch", Summary: "Add missing cases to a type switch or enum switch statement"},
			{Name: "add-tags", Summary: "Add json struct tags to the fields of the struct enclosing the cursor"},
			{Name: "remove-tags", Summary: "Clear struct tags on the fields of the struct enclosing the cursor"},
			{Name: "extract-function", Summary: "Replace the selected statements with a call to a new function"},
			{Name: "extract-variable", Summary: "Replace the selected expression with a reference to a new local variable"},
			{Name: "inline-call", Summary: "Replace a call to a function or method with the contents of its body"},
			{Name: "invert-if", Summary: "Invert an if-else statement, negating its condition and swapping its blocks"},
			{Name: "test", Summary: "Run the Test or Benchmark function nearest to the cursor"},
			{Name: "generate", Summary: "Run go generate for the //go:generate directive nearest to the cursor"},
			{Name: "tidy", Summary: "Run go mod tidy to ensure the go.mod file matches the source code in the module"},
			{Name: "vendor", Summary: "Run go mod vendor to create or update the vendor directory with all necessary dependencies"},
			{Name: "vulncheck", Summary: "Run govulncheck to find known vulnerabilities in functions reachable by the application"},
			{Name: "add-import", Summary: "Add a package import to the current file"},
		},
	}

	return manual, &goRouter{handlers: handlers}, nil
}

var _ textapi.CommandHandler = (*goRouter)(nil)

type goRouter struct {
	handlers map[string]textapi.CommandHandler
}

func (r *goRouter) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	if cmd.Resource == nil {
		return nil
	}
	if cmd.Name != cmdName {
		return fmt.Errorf("unknown command: %s", cmd.Name)
	}
	if len(cmd.Args) == 0 {
		return fmt.Errorf("missing subcommand")
	}
	cmd.Name = cmd.Args[0]
	cmd.Args = cmd.Args[1:]
	h, ok := r.handlers[cmd.Name]
	if !ok {
		return fmt.Errorf("unknown go subcommand: %s", cmd.Name)
	}
	return h.HandleCommand(ctx, cmd)
}

func (r *goRouter) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if cmd != cmdName {
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
	if len(args) == 0 {
		return iterator.Empty[string](), nil
	}
	cmd = args[0]
	args = args[1:]
	if h, ok := r.handlers[cmd]; ok {
		return h.Complete(ctx, cmd, args)
	}
	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return iterator.FromSlice(names), nil
}
