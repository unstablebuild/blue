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
	"github.com/unstablebuild/rune-go-sdk/debug"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// symbolMatch represents a resolved workspace symbol candidate.
type symbolMatch struct {
	URI        string
	Pos        semanticapi.Position
	Display    string
	ImportPath string
}

// symbolProgress reports resolution progress. msg describes the
// current phase, found is the number of candidate matches so far,
// and step/total drive the progress bar.
type symbolProgress func(msg string, found int, step, total int64)

// resolveSymbol resolves a package-qualified symbol name (e.g.
// "fmt.Println") to one or more reference locations by searching the
// workspace with tree-sitter. Results are first deduplicated by file
// URI, then further collapsed by import path so that references to
// the same package produce a single match. When multiple distinct
// packages remain, each gets a display name that disambiguates by
// package path. If progress is non-nil it is called at each phase.
func resolveSymbol(
	ctx context.Context, parser syntaxapi.Parser, name string,
	progress symbolProgress,
) ([]symbolMatch, error) {
	pkg, sym, hasDot := strings.Cut(name, ".")
	if !hasDot {
		return nil, fmt.Errorf("no symbols found for %q", name)
	}

	if progress == nil {
		progress = func(string, int, int64, int64) {}
	}

	seen := make(map[string]bool)
	var matches []symbolMatch

	collect := func(query string, captures []string) error {
		iter, err := parser.Search(query, captures, "go")
		if err != nil {
			return err
		}
		pairs := pairedResults(iter)
		defer func() { _ = pairs.Close() }()
		for {
			p, ok := pairs.Next(ctx)
			if !ok {
				return pairs.Err()
			}
			if p[0].Text != pkg || p[1].Text != sym {
				continue
			}
			uri := p[0].File.String()
			if seen[uri] {
				continue
			}
			seen[uri] = true
			matches = append(matches, symbolMatch{
				URI: uri,
				Pos: semanticapi.Position{
					Line:      uint32(p[1].From.Y),
					Character: uint32(p[1].From.X),
				},
				Display: name,
			})
		}
	}

	progress("Searching types…", 0, 0, 3)
	if err := collect(
		`(qualified_type package: (package_identifier) @pkg name: (type_identifier) @type)`,
		[]string{"pkg", "type"},
	); err != nil {
		return nil, err
	}
	progress("Searching expressions…", len(matches), 1, 3)
	if err := collect(
		`(selector_expression operand: (identifier) @pkg field: (field_identifier) @symbol)`,
		[]string{"pkg", "symbol"},
	); err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no symbols found for %q", name)
	}
	if len(matches) > 1 {
		progress("Resolving imports…", len(matches), 2, 3)
		matches = deduplicateByImport(ctx, parser, matches, pkg)
	}
	if len(matches) > 1 {
		disambiguateDisplayNames(matches, name)
	}
	return matches, nil
}

// disambiguateDisplayNames sets display names that differentiate
// matches from different packages. It prefers the resolved Go import
// path and falls back to a package path derived from the file URI.
func disambiguateDisplayNames(matches []symbolMatch, name string) {
	for i, m := range matches {
		prefix := m.ImportPath
		if prefix == "" {
			prefix = packagePathFromURI(m.URI)
		}
		matches[i].Display = prefix + ": " + name
	}
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

// resolveImportPaths builds a per-file alias→import-path lookup
// table by scanning all import declarations in the workspace.
func resolveImportPaths(
	ctx context.Context, parser syntaxapi.Parser,
) (map[workspaceapi.URI]map[string]string, error) {
	type fileImports = map[workspaceapi.URI][]string
	type fileAliases = map[workspaceapi.URI]map[string]string

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
			if m == nil {
				m = make(fileImports)
			}
			m[r.File] = append(m[r.File], p)
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
	explicitAliases, err := iterator.Reduce(ctx, pairedResults(aliasIter),
		func(m fileAliases, p [2]syntaxapi.Result) (fileAliases, error) {
			alias := p[0].Text
			if alias == "." || alias == "_" {
				return m, nil
			}
			importPath := strings.Trim(p[1].Text, `"`)
			if m == nil {
				m = make(fileAliases)
			}
			if m[p[0].File] == nil {
				m[p[0].File] = make(map[string]string)
			}
			m[p[0].File][importPath] = alias
			return m, nil
		},
	)
	if err != nil {
		return nil, err
	}
	resolved := make(map[workspaceapi.URI]map[string]string, len(imports))
	for file, paths := range imports {
		aliases := make(map[string]string)
		for _, importPath := range paths {
			alias := path.Base(importPath)
			if fileAliases := explicitAliases[file]; fileAliases != nil {
				if explicitAlias, ok := fileAliases[importPath]; ok {
					alias = explicitAlias
				}
			}
			if alias == "." || alias == "_" {
				continue
			}
			aliases[alias] = importPath
		}
		if len(aliases) > 0 {
			resolved[file] = aliases
		}
	}
	return resolved, nil
}

// deduplicateByImport collapses matches that reference the same
// import path for the given alias into a single match.
func deduplicateByImport(
	ctx context.Context, parser syntaxapi.Parser,
	matches []symbolMatch, alias string,
) []symbolMatch {
	importPaths, err := resolveImportPaths(ctx, parser)
	if err != nil {
		return matches
	}
	lookup := make(map[string]map[string]string, len(importPaths))
	for uri, aliases := range importPaths {
		lookup[uri.String()] = aliases
	}
	seen := make(map[string]bool)
	result := matches[:0]
	for _, m := range matches {
		key := m.URI
		if fileImports := lookup[m.URI]; fileImports != nil {
			if importPath, ok := fileImports[alias]; ok {
				key = importPath
				m.ImportPath = importPath
			}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, m)
	}
	return result
}

// resolveCommandSymbol handles the common symbol-resolution pattern for
// command handlers. If cmd has no args and a resource, proceed=true is
// returned so the handler can use the cursor position. If cmd has args,
// the symbol is resolved asynchronously: a progress notification is
// shown, resolution runs in a background goroutine, and onResolve is
// called on the main thread (via scheduleNextTick) when a single match
// is found. When multiple matches exist a picker is shown instead.
// In the async case proceed=false is always returned.
func resolveCommandSymbol(
	_ context.Context, cmd *textapi.Command,
	wm browserapi.WindowManager,
	fs workspaceapi.FileSystem,
	notify browserapi.Notifications,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser,
	onResolve func(symbolMatch),
) (proceed bool, err error) {
	if len(cmd.Args) == 0 {
		if cmd.Resource == nil {
			return false, fmt.Errorf("no file open; pass a symbol name or open a file first")
		}
		return true, nil
	}
	name := strings.Join(cmd.Args, " ")

	id, _ := notify.Notify(browserapi.LevelInfo, "Resolving %s…", name)
	_ = notify.UpdateNotificationProgress(id, "", 0, 3)

	go debug.CapturePanicReport(func() {
		progress := func(msg string, found int, step, total int64) {
			if found > 0 {
				msg = fmt.Sprintf("%s (%d found)", msg, found)
			}
			_ = notify.UpdateNotificationProgress(id, msg, step, total)
		}
		matches, err := resolveSymbol(context.Background(), parser, name, progress)
		scheduleNextTick(func() {
			_ = notify.UpdateNotificationProgress(id, "", 3, 3)
			if err != nil {
				_, _ = notify.Notify(browserapi.LevelError, "%s", err)
				return
			}
			if len(matches) == 1 {
				onResolve(matches[0])
				return
			}
			if err := showSymbolPicker(matches, wm, fs, scheduleNextTick, parser, onResolve); err != nil {
				_, _ = notify.Notify(browserapi.LevelError, "%s", err)
			}
		})
	})

	return false, nil
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
		fs, scheduleNextTick, parser, DefaultLocationsConfig(), nil,
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
