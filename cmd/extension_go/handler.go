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

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

const cmdName = "go"

func newGoHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, notify browserapi.Notifications,
) (textapi.CommandManual, textapi.CommandHandler) {
	handlers := map[string]textapi.CommandHandler{
		// Source actions.
		"organize-imports": codeActionHandler(lsp, editor, notify, wm, "source.organizeImports",
			"Imports are already organized"),
		"fix-all": codeActionHandler(lsp, editor, notify, wm, "source.fixAll",
			"No automatic fixes available"),
		"add-test": codeActionHandler(lsp, editor, notify, wm, "source.addTest",
			"Place cursor inside a function to generate a test"),
		"assembly": codeActionHandler(lsp, editor, notify, wm, "source.assembly",
			"Place cursor inside a function to view assembly. Generic functions and init() are not supported"),
		"doc": codeActionHandler(lsp, editor, notify, wm, "source.doc",
			"No documentation available for this package"),
		"free-symbols": codeActionHandler(lsp, editor, notify, wm, "source.freesymbols",
			"Select a block of code to analyze its free symbols"),

		// Refactoring actions.
		"fill-struct": codeActionHandler(lsp, editor, notify, wm, "refactor.rewrite.fillStruct",
			"Place cursor on a struct literal (e.g. Type{}) to fill fields"),
		"fill-switch": codeActionHandler(lsp, editor, notify, wm, "refactor.rewrite.fillSwitch",
			"Place cursor on a switch statement to add missing cases"),
		"add-tags": codeActionHandler(lsp, editor, notify, wm, "refactor.rewrite.addTags",
			"Place cursor on a struct field to add tags"),
		"remove-tags": codeActionHandler(lsp, editor, notify, wm, "refactor.rewrite.removeTags",
			"Place cursor on a struct field with existing tags"),
		"extract-function": codeActionHandler(lsp, editor, notify, wm, "refactor.extract.function",
			"Select one or more complete statements to extract"),
		"extract-variable": codeActionHandler(lsp, editor, notify, wm, "refactor.extract.variable",
			"Select an expression to extract as a variable"),
		"inline-call": codeActionHandler(lsp, editor, notify, wm, "refactor.inline.call",
			"Place cursor on a function call to inline it"),
		"invert-if": codeActionHandler(lsp, editor, notify, wm, "refactor.rewrite.invertIf",
			"Place cursor on an if-else statement to invert its condition"),

		// Code lens commands.
		"test":     codeLensHandler(lsp, notify, "gopls.run_tests"),
		"generate": codeLensHandler(lsp, notify, "gopls.generate"),

		// Module management.
		"tidy":      modCommandHandler(lsp, notify, "gopls.tidy"),
		"vendor":    modCommandHandler(lsp, notify, "gopls.vendor"),
		"vulncheck": modCommandHandler(lsp, notify, "gopls.run_govulncheck"),

		// Imports.
		"add-import": addImportHandler(lsp, notify),
	}

	manual := textapi.CommandManual{
		Name:     cmdName,
		Summary:  "Go language commands powered by gopls",
		Synopsis: "<subcommand> [<args>...]",
		Commands: []textapi.CommandManual{
			{Name: "organize-imports", Summary: "Sort, add missing, remove unused imports"},
			{Name: "fix-all", Summary: "Apply all safe automatic fixes"},
			{Name: "add-test", Summary: "Generate test for function at cursor"},
			{Name: "assembly", Summary: "View assembly for function at cursor"},
			{Name: "doc", Summary: "View package documentation"},
			{Name: "free-symbols", Summary: "Browse free symbols in selection"},
			{Name: "fill-struct", Summary: "Fill struct literal with zero-value fields"},
			{Name: "fill-switch", Summary: "Add missing cases to type switch"},
			{Name: "add-tags", Summary: "Add struct field tags (e.g. json)"},
			{Name: "remove-tags", Summary: "Remove struct field tags"},
			{Name: "extract-function", Summary: "Extract selection to new function"},
			{Name: "extract-variable", Summary: "Extract expression to variable"},
			{Name: "inline-call", Summary: "Inline function call at cursor"},
			{Name: "invert-if", Summary: "Invert if/else condition"},
			{Name: "test", Summary: "Run test/benchmark at cursor via code lens"},
			{Name: "generate", Summary: "Run go generate for directive at cursor"},
			{Name: "tidy", Summary: "Run go mod tidy"},
			{Name: "vendor", Summary: "Run go mod vendor"},
			{Name: "vulncheck", Summary: "Run vulnerability check"},
			{Name: "add-import", Summary: "Add import path to file"},
		},
	}

	return manual, &goRouter{handlers: handlers}
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
