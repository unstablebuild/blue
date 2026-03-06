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

// symbolMatch represents a resolved workspace symbol candidate.
type symbolMatch struct {
	URI     string
	Pos     semanticapi.Position
	Display string
}

// resolveSymbol resolves a symbol name to one or more candidates by querying
// the workspace symbol provider. Exact name matches are preferred and
// deduplicated by URI. When multiple candidates remain, each gets a display
// name that disambiguates by package.
func resolveSymbol(
	ctx context.Context, lsp semanticapi.LSP, name string,
) ([]symbolMatch, error) {
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
		return []symbolMatch{{
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
		return []symbolMatch{{
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
func symbolDisplayNames(syms []semanticapi.SymbolInformation) []symbolMatch {
	matches := make([]symbolMatch, len(syms))
	pkgPaths := make([]string, len(syms))
	shortPkgs := make([]string, len(syms))
	for i, s := range syms {
		matches[i] = symbolMatch{
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
	onPick func(symbolMatch),
) (proceed bool, err error) {
	if len(cmd.Args) == 0 {
		return cmd.Resource != nil, nil
	}
	matches, err := resolveSymbol(ctx, lsp, strings.Join(cmd.Args, " "))
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
	matches []symbolMatch,
	wm browserapi.WindowManager,
	fs workspaceapi.FileSystem,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser,
	onPick func(symbolMatch),
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
	win, err := wm.Floating(handler, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		return err
	}
	handler.win = win
	return nil
}



// completeReferencedSymbol returns package-qualified symbol names
// referenced across the workspace by running tree-sitter queries to
// find imports and their usages (qualified types and selector
// expressions). This provides a richer fallback than document symbols
// for empty-query completion because it includes dependency symbols.
//
// All I/O runs in a background goroutine. The returned iterator
// streams results through a channel so the calling goroutine never
// blocks on parser queries.
func completeReferencedSymbol(
	ctx context.Context, parser syntaxapi.Parser,
) (iterator.Iterator[string], error) {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan string)
	errc := make(chan error, 1)

	go func() {
		defer close(ch)
		if err := produceReferencedSymbols(ctx, parser, ch); err != nil {
			errc <- err
		}
	}()

	seen := make(map[string]bool)
	return iterator.FromFunc(func(ctx context.Context) (string, bool, error) {
		for {
			select {
			case s, ok := <-ch:
				if !ok {
					select {
					case err := <-errc:
						return "", false, err
					default:
						return "", false, nil
					}
				}
				if seen[s] {
					continue
				}
				seen[s] = true
				return s, true, nil
			case <-ctx.Done():
				return "", false, ctx.Err()
			}
		}
	}, func() error {
		cancel()
		for range ch {
		}
		select {
		case err := <-errc:
			return err
		default:
			return nil
		}
	}), nil
}

// produceReferencedSymbols runs in a background goroutine. It first
// reduces all imports into a lookup table, then streams qualified-type
// and selector-expression matches to ch.
func produceReferencedSymbols(
	ctx context.Context, parser syntaxapi.Parser, ch chan<- string,
) error {
	imports, err := reduceImports(ctx, parser)
	if err != nil {
		return err
	}

	if err := searchPairs(ctx, parser,
		`(qualified_type package: (package_identifier) @pkg name: (type_identifier) @type)`,
		[]string{"pkg", "type"}, "go",
		func(p [2]syntaxapi.Result) bool { return isExported(p[1].Text) },
		ch,
	); err != nil {
		return err
	}

	return searchPairs(ctx, parser,
		`(selector_expression operand: (identifier) @pkg field: (field_identifier) @symbol)`,
		[]string{"pkg", "symbol"}, "go",
		func(p [2]syntaxapi.Result) bool {
			if !isExported(p[1].Text) {
				return false
			}
			m := imports[p[0].File]
			return m != nil && m[p[0].Text]
		},
		ch,
	)
}

// searchPairs runs a two-capture tree-sitter search, pairs
// consecutive captures, applies keep as a filter, and sends
// the resulting "pkg.Name" strings to ch.
func searchPairs(
	ctx context.Context, parser syntaxapi.Parser,
	query string, captures []string, lang string,
	keep func([2]syntaxapi.Result) bool,
	ch chan<- string,
) error {
	iter, err := parser.Search(query, captures, lang)
	if err != nil {
		return err
	}
	results := iterator.Map(
		iterator.Filter(pairedResults(iter), keep),
		func(p [2]syntaxapi.Result) string {
			return p[0].Text + "." + p[1].Text
		},
	)
	defer func() { _ = results.Close() }()
	for {
		s, ok := results.Next(ctx)
		if !ok {
			return results.Err()
		}
		select {
		case ch <- s:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// reduceImports builds a per-file import alias lookup table by
// reducing two tree-sitter queries: one for all import paths
// (deriving the default alias) and one for explicit import names
// (overriding the default).
func reduceImports(
	ctx context.Context, parser syntaxapi.Parser,
) (map[workspaceapi.URI]map[string]bool, error) {
	type fileImports = map[workspaceapi.URI]map[string]bool

	pathIter, err := parser.Search(
		`(import_spec path: (interpreted_string_literal) @path)`,
		[]string{"path"}, "go",
	)
	if err != nil {
		return nil, err
	}
	imports, err := iterator.Reduce(ctx, pathIter,
		func(m fileImports, r syntaxapi.Result) (fileImports, error) {
			p := strings.Trim(r.Text, `"`)
			alias := path.Base(p)
			if alias == "." || alias == "_" {
				return m, nil
			}
			if m == nil {
				m = make(fileImports)
			}
			if m[r.File] == nil {
				m[r.File] = make(map[string]bool)
			}
			m[r.File][alias] = true
			return m, nil
		},
	)
	if err != nil {
		return nil, err
	}

	aliasIter, err := parser.Search(
		`(import_spec name: (package_identifier) @alias path: (interpreted_string_literal) @path)`,
		[]string{"alias", "path"}, "go",
	)
	if err != nil {
		return nil, err
	}
	// Mutate imports in place; the Reduce accumulator is unused.
	_, err = iterator.Reduce(ctx, pairedResults(aliasIter),
		func(_ struct{}, p [2]syntaxapi.Result) (struct{}, error) {
			alias := p[0].Text
			if alias == "." || alias == "_" || imports == nil {
				return struct{}{}, nil
			}
			defaultAlias := path.Base(strings.Trim(p[1].Text, `"`))
			if fm := imports[p[0].File]; fm != nil {
				delete(fm, defaultAlias)
				fm[alias] = true
			}
			return struct{}{}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	if imports == nil {
		imports = make(fileImports)
	}
	return imports, nil
}

// pairedResults groups results from a two-capture tree-sitter query
// into pairs. Because the gRPC stream may interleave results from
// files processed concurrently, consecutive results are not
// guaranteed to belong to the same match. This function buffers the
// first capture per file and pairs it with the next result from the
// same file.
func pairedResults(
	it iterator.Iterator[syntaxapi.Result],
) iterator.Iterator[[2]syntaxapi.Result] {
	pending := make(map[workspaceapi.URI]syntaxapi.Result)
	return iterator.FromFunc(
		func(ctx context.Context) ([2]syntaxapi.Result, bool, error) {
			for {
				r, ok := it.Next(ctx)
				if !ok {
					return [2]syntaxapi.Result{}, false, it.Err()
				}
				if first, exists := pending[r.File]; exists {
					delete(pending, r.File)
					return [2]syntaxapi.Result{first, r}, true, nil
				}
				pending[r.File] = r
			}
		}, it.Close,
	)
}

func isExported(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
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
