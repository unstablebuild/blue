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

package lspcmd

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// SymbolMatch represents a resolved workspace symbol candidate.
type SymbolMatch struct {
	URI     string
	Pos     semanticapi.Position
	Display string
}

// ResolveSymbol resolves a symbol name to one or more candidates by querying
// the workspace symbol provider. Exact name matches are preferred and
// deduplicated by URI. When multiple candidates remain, each gets a display
// name that disambiguates by package.
func ResolveSymbol(
	ctx context.Context, lsp semanticapi.LSP, name string,
) ([]SymbolMatch, error) {
	syms, err := lsp.WorkspaceSymbol(ctx, semanticapi.WorkspaceSymbolParams{
		Query: name,
	})
	if err != nil {
		return nil, err
	}
	if len(syms) == 0 {
		return nil, fmt.Errorf("no symbols found for %q", name)
	}
	// Prefer exact name matches.
	var exact []semanticapi.SymbolInformation
	for _, s := range syms {
		if s.Name == name {
			exact = append(exact, s)
		}
	}
	if len(exact) == 0 {
		// Fall back to the first result (best fuzzy match).
		s := syms[0]
		return []SymbolMatch{{
			URI:     s.Location.URI,
			Pos:     s.Location.Range.Start,
			Display: s.Name,
		}}, nil
	}
	// Deduplicate exact matches by URI.
	seen := make(map[string]bool)
	var deduped []semanticapi.SymbolInformation
	for _, s := range exact {
		if !seen[s.Location.URI] {
			seen[s.Location.URI] = true
			deduped = append(deduped, s)
		}
	}
	if len(deduped) == 1 {
		s := deduped[0]
		return []SymbolMatch{{
			URI:     s.Location.URI,
			Pos:     s.Location.Range.Start,
			Display: s.Name,
		}}, nil
	}
	return symbolDisplayNames(deduped), nil
}

// symbolDisplayNames computes display names for a set of symbols.
// It uses <package>.<symbol> when the short package name is unique,
// and the full package path when there are collisions.
func symbolDisplayNames(syms []semanticapi.SymbolInformation) []SymbolMatch {
	matches := make([]SymbolMatch, len(syms))
	pkgPaths := make([]string, len(syms))
	shortPkgs := make([]string, len(syms))
	for i, s := range syms {
		matches[i] = SymbolMatch{
			URI: s.Location.URI,
			Pos: s.Location.Range.Start,
		}
		pkgPaths[i] = packagePathFromURI(s.Location.URI)
		shortPkgs[i] = path.Base(pkgPaths[i])
	}
	shortCount := make(map[string]int)
	for _, sp := range shortPkgs {
		shortCount[sp]++
	}
	for i, s := range syms {
		if shortCount[shortPkgs[i]] > 1 {
			matches[i].Display = pkgPaths[i] + "." + s.Name
		} else {
			matches[i].Display = shortPkgs[i] + "." + s.Name
		}
	}
	return matches
}

// packagePathFromURI extracts a Go-style package path from a file:// URI.
// For module cache files it extracts the import path; otherwise it returns
// the directory portion of the path.
func packagePathFromURI(uri string) string {
	p := strings.TrimPrefix(uri, "file://")
	dir := path.Dir(p)
	if idx := strings.Index(dir, "/pkg/mod/"); idx != -1 {
		modPath := dir[idx+len("/pkg/mod/"):]
		parts := strings.Split(modPath, "/")
		for i, part := range parts {
			if atIdx := strings.Index(part, "@"); atIdx != -1 {
				parts[i] = part[:atIdx]
			}
		}
		return strings.Join(parts, "/")
	}
	return dir
}

// resolveCommandSymbol handles the common symbol-resolution pattern for
// command handlers. If cmd has args, it resolves the symbol name. When
// exactly one match is found, cmd.URI and cmd.Cursor.Content are set and
// proceed=true is returned. When multiple matches are found, a picker is
// shown and proceed=false is returned. When cmd has no args and no resource,
// proceed=false is returned.
func resolveCommandSymbol(
	ctx context.Context, cmd *textapi.Command,
	lsp semanticapi.LSP, wm browserapi.WindowManager,
	fs workspaceapi.FileSystem,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser,
	onPick func(SymbolMatch),
) (proceed bool, err error) {
	if len(cmd.Args) == 0 {
		return cmd.Resource != nil, nil
	}
	matches, err := ResolveSymbol(ctx, lsp, strings.Join(cmd.Args, " "))
	if err != nil {
		return false, err
	}
	if len(matches) > 1 {
		return false, showSymbolPicker(matches, wm, fs, scheduleNextTick, parser, onPick)
	}
	wsURI, err := LspToURI(matches[0].URI)
	if err != nil {
		return false, err
	}
	cmd.URI = wsURI
	cmd.Cursor.Content = PosToCoord(matches[0].Pos)
	return true, nil
}

// showSymbolPicker displays a floating locations picker with file preview
// for the given symbol matches. When the user selects a match, onPick is
// called. It reuses the locationsFloatingHandler with a custom onSelect.
func showSymbolPicker(
	matches []SymbolMatch,
	wm browserapi.WindowManager,
	fs workspaceapi.FileSystem,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser,
	onPick func(SymbolMatch),
) error {
	entries := make([]locationEntry, len(matches))
	for i, m := range matches {
		entries[i] = locationEntry{
			uri:     m.URI,
			rng:     semanticapi.Range{Start: m.Pos, End: m.Pos},
			display: m.Display,
		}
	}
	handler := newLocationsFloatingHandler(
		entries, nil, wm, nil, nil,
		fs, scheduleNextTick, parser, DefaultLocationsConfig(),
	)
	handler.onSelect = func(idx int) {
		onPick(matches[idx])
	}
	_, err := wm.Floating(handler, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	return err
}

// CompleteSymbol returns symbol-name completions from the workspace symbol
// provider. The arg is forwarded as the query to the workspace symbol request.
func CompleteSymbol(
	ctx context.Context, lsp semanticapi.LSP, arg string,
) (iterator.Iterator[string], error) {
	syms, err := lsp.WorkspaceSymbol(ctx, semanticapi.WorkspaceSymbolParams{
		Query: arg,
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(syms))
	for i, s := range syms {
		names[i] = s.Name
	}
	return iterator.FromSlice(names), nil
}

// CompleteDocumentSymbol returns symbol-name completions from the document
// symbol provider for the given URI. This is used as a fallback when the
// workspace symbol query is empty, since gopls returns nothing for empty
// workspace/symbol queries.
func CompleteDocumentSymbol(
	ctx context.Context, lsp semanticapi.LSP, uri workspaceapi.URI,
) (iterator.Iterator[string], error) {
	result, err := lsp.DocumentSymbol(ctx, semanticapi.DocumentSymbolParams{
		TextDocument: TextDocID(uri),
	})
	if err != nil {
		return nil, err
	}
	var names []string
	collectDocSymbolNames(&names, result.DocumentSymbols)
	for _, s := range result.SymbolInformation {
		names = append(names, normalizeMethodName(s.Name))
	}
	return iterator.FromSlice(names), nil
}

func collectDocSymbolNames(names *[]string, syms []semanticapi.DocumentSymbol) {
	for _, s := range syms {
		*names = append(*names, normalizeMethodName(s.Name))
		collectDocSymbolNames(names, s.Children)
	}
}

// normalizeMethodName converts gopls document-symbol receiver method
// names like "(*Type).Method" or "(Type).Method" to "Type.Method" so
// they match the format used by workspace/symbol.
func normalizeMethodName(name string) string {
	if after, ok := strings.CutPrefix(name, "(*"); ok {
		if i := strings.Index(after, ")."); i != -1 {
			return after[:i] + "." + after[i+2:]
		}
	}
	if after, ok := strings.CutPrefix(name, "("); ok {
		if i := strings.Index(after, ")."); i != -1 {
			return after[:i] + "." + after[i+2:]
		}
	}
	return name
}
