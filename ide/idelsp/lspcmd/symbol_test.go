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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestResolveSymbol(t *testing.T) {
	t.Parallel()

	fileA, err := workspaceapi.ParseURI("file:///project/pkg/a.go")
	require.NoError(t, err)
	fileB, err := workspaceapi.ParseURI("file:///project/pkg/b.go")
	require.NoError(t, err)
	fileC, err := workspaceapi.ParseURI("file:///project/other/c.go")
	require.NoError(t, err)

	tests := []struct {
		name        string
		query       string
		types       []syntaxapi.Result
		selectors   []syntaxapi.Result
		wantMatches []symbolMatch
		wantErr     bool
	}{
		{
			name:  "qualified type found",
			query: "context.Context",
			types: []syntaxapi.Result{
				{File: fileA, Text: "context", From: term.Coordinates{X: 5, Y: 10}, CaptureName: "pkg"},
				{File: fileA, Text: "Context", From: term.Coordinates{X: 13, Y: 10}, CaptureName: "type"},
			},
			wantMatches: []symbolMatch{{
				URI:     fileA.String(),
				Pos:     semanticapi.Position{Line: 10, Character: 13},
				Display: "context.Context",
			}},
		},
		{
			name:  "selector expression found",
			query: "fmt.Println",
			selectors: []syntaxapi.Result{
				{File: fileA, Text: "fmt", From: term.Coordinates{X: 1, Y: 20}, CaptureName: "pkg"},
				{File: fileA, Text: "Println", From: term.Coordinates{X: 5, Y: 20}, CaptureName: "symbol"},
			},
			wantMatches: []symbolMatch{{
				URI:     fileA.String(),
				Pos:     semanticapi.Position{Line: 20, Character: 5},
				Display: "fmt.Println",
			}},
		},
		{
			name:    "no dot in name errors",
			query:   "NoDot",
			wantErr: true,
		},
		{
			name:    "no results errors",
			query:   "missing.Symbol",
			wantErr: true,
		},
		{
			name:  "multiple files show picker with display names",
			query: "fmt.Println",
			selectors: []syntaxapi.Result{
				{File: fileA, Text: "fmt", From: term.Coordinates{X: 1, Y: 5}, CaptureName: "pkg"},
				{File: fileA, Text: "Println", From: term.Coordinates{X: 5, Y: 5}, CaptureName: "symbol"},
				{File: fileC, Text: "fmt", From: term.Coordinates{X: 1, Y: 10}, CaptureName: "pkg"},
				{File: fileC, Text: "Println", From: term.Coordinates{X: 5, Y: 10}, CaptureName: "symbol"},
			},
			wantMatches: []symbolMatch{
				{URI: fileA.String(), Pos: semanticapi.Position{Line: 5, Character: 5}, Display: "pkg: fmt.Println"},
				{URI: fileC.String(), Pos: semanticapi.Position{Line: 10, Character: 5}, Display: "other: fmt.Println"},
			},
		},
		{
			name:  "duplicate URIs deduplicated",
			query: "context.Context",
			types: []syntaxapi.Result{
				{File: fileA, Text: "context", From: term.Coordinates{X: 5, Y: 10}, CaptureName: "pkg"},
				{File: fileA, Text: "Context", From: term.Coordinates{X: 13, Y: 10}, CaptureName: "type"},
				{File: fileA, Text: "context", From: term.Coordinates{X: 5, Y: 20}, CaptureName: "pkg"},
				{File: fileA, Text: "Context", From: term.Coordinates{X: 13, Y: 20}, CaptureName: "type"},
			},
			wantMatches: []symbolMatch{{
				URI:     fileA.String(),
				Pos:     semanticapi.Position{Line: 10, Character: 13},
				Display: "context.Context",
			}},
		},
		{
			name:  "non-matching pairs skipped",
			query: "fmt.Println",
			selectors: []syntaxapi.Result{
				{File: fileA, Text: "os", From: term.Coordinates{X: 1, Y: 5}, CaptureName: "pkg"},
				{File: fileA, Text: "Exit", From: term.Coordinates{X: 4, Y: 5}, CaptureName: "symbol"},
				{File: fileB, Text: "fmt", From: term.Coordinates{X: 1, Y: 8}, CaptureName: "pkg"},
				{File: fileB, Text: "Println", From: term.Coordinates{X: 5, Y: 8}, CaptureName: "symbol"},
			},
			wantMatches: []symbolMatch{{
				URI:     fileB.String(),
				Pos:     semanticapi.Position{Line: 8, Character: 5},
				Display: "fmt.Println",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := &mockParser{
				searchFn: func(query string, _ []string) (iterator.Iterator[syntaxapi.Result], error) {
					if strings.Contains(query, "qualified_type") {
						return iterator.FromSlice(tt.types), nil
					}
					if strings.Contains(query, "selector_expression") {
						return iterator.FromSlice(tt.selectors), nil
					}
					return iterator.Empty[syntaxapi.Result](), nil
				},
			}
			matches, err := resolveSymbol(context.Background(), parser, tt.query)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMatches, matches)
		})
	}
}

func TestPackagePathFromURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		uri  string
		want string
	}{
		{
			uri:  "file:///Users/foo/go/pkg/mod/github.com/org/repo@v1.2.3/cell/buffer.go",
			want: "github.com/org/repo/cell",
		},
		{
			uri:  "file:///project/internal/cell/buffer.go",
			want: "/project/internal/cell",
		},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, packagePathFromURI(tt.uri))
		})
	}
}

func TestNormalizeMethodName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		// Pointer receiver methods — the core case.
		{
			name: "simple pointer receiver",
			in:   "(*Greeter).Greet",
			want: "Greeter.Greet",
		},
		{
			name: "pointer receiver with short name",
			in:   "(*T).M",
			want: "T.M",
		},
		{
			name: "pointer receiver with long type name",
			in:   "(*definitionHandler).HandleCommand",
			want: "definitionHandler.HandleCommand",
		},
		{
			name: "pointer receiver with unexported type",
			in:   "(*routerHandler).Handle",
			want: "routerHandler.Handle",
		},
		{
			name: "pointer receiver with underscore type",
			in:   "(*my_type).do_thing",
			want: "my_type.do_thing",
		},
		{
			name: "pointer receiver with numeric suffix",
			in:   "(*Handler2).ServeHTTP",
			want: "Handler2.ServeHTTP",
		},

		// Value receiver methods.
		{
			name: "value receiver",
			in:   "(Counter).Count",
			want: "Counter.Count",
		},
		{
			name: "value receiver short",
			in:   "(T).String",
			want: "T.String",
		},
		{
			name: "value receiver with unexported type",
			in:   "(myStruct).value",
			want: "myStruct.value",
		},

		// Non-method symbols — should pass through unchanged.
		{
			name: "plain function",
			in:   "Add",
			want: "Add",
		},
		{
			name: "plain type",
			in:   "Greeter",
			want: "Greeter",
		},
		{
			name: "constant",
			in:   "MaxSize",
			want: "MaxSize",
		},
		{
			name: "qualified symbol",
			in:   "mylib.MyType",
			want: "mylib.MyType",
		},
		{
			name: "value receiver method already normalized",
			in:   "Counter.Count",
			want: "Counter.Count",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},

		// Edge cases — malformed but shouldn't panic.
		{
			name: "open paren star without close",
			in:   "(*Foo",
			want: "(*Foo",
		},
		{
			name: "only prefix",
			in:   "(*",
			want: "(*",
		},
		{
			name: "paren star with close but no dot",
			in:   "(*Foo)",
			want: "(*Foo)",
		},
		{
			name: "nested parens are not mangled",
			in:   "func(*int)",
			want: "func(*int)",
		},
		{
			name: "open paren without close",
			in:   "(Foo",
			want: "(Foo",
		},
		{
			name: "paren with close but no dot",
			in:   "(Foo)",
			want: "(Foo)",
		},
		{
			name: "star without paren prefix",
			in:   "*Foo.Bar",
			want: "*Foo.Bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, normalizeMethodName(tt.in))
		})
	}
}

func TestCompleteReferencedSymbol(t *testing.T) {
	t.Parallel()

	// URIs mirror the format returned by syntaxapi.Parser.Search():
	// file:// + workspace root + relative path.
	fileA, err := workspaceapi.ParseURI(
		"file:///workspace/workspace/ide/idelsp/lspcmd/symbol.go",
	)
	require.NoError(t, err)
	fileB, err := workspaceapi.ParseURI(
		"file:///workspace/workspace/ide/idelsp/lspcmd/commands.go",
	)
	require.NoError(t, err)

	type queryResults struct {
		imports   []syntaxapi.Result
		aliases   []syntaxapi.Result
		types     []syntaxapi.Result
		selectors []syntaxapi.Result
	}
	searchRouter := func(qr queryResults) func(string, []string) (iterator.Iterator[syntaxapi.Result], error) {
		return func(query string, _ []string) (iterator.Iterator[syntaxapi.Result], error) {
			switch {
			case strings.Contains(query, "import_spec") && !strings.Contains(query, "name:"):
				return iterator.FromSlice(qr.imports), nil
			case strings.Contains(query, "import_spec") && strings.Contains(query, "name:"):
				return iterator.FromSlice(qr.aliases), nil
			case strings.Contains(query, "qualified_type"):
				return iterator.FromSlice(qr.types), nil
			case strings.Contains(query, "selector_expression"):
				return iterator.FromSlice(qr.selectors), nil
			}
			return iterator.Empty[syntaxapi.Result](), nil
		}
	}

	// Result data below is sampled from actual syntax_query output
	// on real Go files. The From/To coordinates, File URIs, and Text
	// values match the shapes returned by syntaxapi.Parser.Search().

	tests := []struct {
		name    string
		results queryResults
		want    []string
	}{
		{
			name: "realistic Go file with mixed types and selectors",
			results: queryResults{
				imports: []syntaxapi.Result{
					{File: fileA, Text: `"context"`, From: term.Coordinates{X: 1, Y: 26}, To: term.Coordinates{X: 10, Y: 26}, CaptureName: "path"},
					{File: fileA, Text: `"fmt"`, From: term.Coordinates{X: 1, Y: 27}, To: term.Coordinates{X: 6, Y: 27}, CaptureName: "path"},
					{File: fileA, Text: `"strings"`, From: term.Coordinates{X: 1, Y: 29}, To: term.Coordinates{X: 10, Y: 29}, CaptureName: "path"},
					{File: fileA, Text: `"github.com/unstablebuild/rune-go-sdk/api/semanticapi"`, From: term.Coordinates{X: 1, Y: 32}, To: term.Coordinates{X: 55, Y: 32}, CaptureName: "path"},
				},
				types: []syntaxapi.Result{
					// context.Context at line 52
					{File: fileA, Text: "context", From: term.Coordinates{X: 5, Y: 52}, To: term.Coordinates{X: 12, Y: 52}, CaptureName: "pkg"},
					{File: fileA, Text: "Context", From: term.Coordinates{X: 13, Y: 52}, To: term.Coordinates{X: 20, Y: 52}, CaptureName: "type"},
					// semanticapi.LSP at line 52
					{File: fileA, Text: "semanticapi", From: term.Coordinates{X: 26, Y: 52}, To: term.Coordinates{X: 37, Y: 52}, CaptureName: "pkg"},
					{File: fileA, Text: "LSP", From: term.Coordinates{X: 38, Y: 52}, To: term.Coordinates{X: 41, Y: 52}, CaptureName: "type"},
					// semanticapi.Position at line 43
					{File: fileA, Text: "semanticapi", From: term.Coordinates{X: 9, Y: 43}, To: term.Coordinates{X: 20, Y: 43}, CaptureName: "pkg"},
					{File: fileA, Text: "Position", From: term.Coordinates{X: 21, Y: 43}, To: term.Coordinates{X: 29, Y: 43}, CaptureName: "type"},
				},
				selectors: []syntaxapi.Result{
					// fmt.Errorf — valid package call
					{File: fileA, Text: "fmt", From: term.Coordinates{X: 14, Y: 61}, To: term.Coordinates{X: 17, Y: 61}, CaptureName: "pkg"},
					{File: fileA, Text: "Errorf", From: term.Coordinates{X: 18, Y: 61}, To: term.Coordinates{X: 24, Y: 61}, CaptureName: "symbol"},
					// s.Name — struct field access, "s" is not an import
					{File: fileA, Text: "s", From: term.Coordinates{X: 5, Y: 66}, To: term.Coordinates{X: 6, Y: 66}, CaptureName: "pkg"},
					{File: fileA, Text: "Name", From: term.Coordinates{X: 7, Y: 66}, To: term.Coordinates{X: 11, Y: 66}, CaptureName: "symbol"},
					// s.Location — struct field access
					{File: fileA, Text: "s", From: term.Coordinates{X: 12, Y: 74}, To: term.Coordinates{X: 13, Y: 74}, CaptureName: "pkg"},
					{File: fileA, Text: "Location", From: term.Coordinates{X: 14, Y: 74}, To: term.Coordinates{X: 22, Y: 74}, CaptureName: "symbol"},
					// strings.TrimPrefix — valid package call
					{File: fileA, Text: "strings", From: term.Coordinates{X: 6, Y: 132}, To: term.Coordinates{X: 13, Y: 132}, CaptureName: "pkg"},
					{File: fileA, Text: "TrimPrefix", From: term.Coordinates{X: 14, Y: 132}, To: term.Coordinates{X: 24, Y: 132}, CaptureName: "symbol"},
					// strings.Index — valid package call (duplicate pkg)
					{File: fileA, Text: "strings", From: term.Coordinates{X: 11, Y: 134}, To: term.Coordinates{X: 18, Y: 134}, CaptureName: "pkg"},
					{File: fileA, Text: "Index", From: term.Coordinates{X: 19, Y: 134}, To: term.Coordinates{X: 24, Y: 134}, CaptureName: "symbol"},
				},
			},
			want: []string{
				"context.Context", "semanticapi.LSP", "semanticapi.Position",
				"fmt.Errorf", "strings.TrimPrefix", "strings.Index",
			},
		},
		{
			name: "explicit alias overrides default",
			results: queryResults{
				imports: []syntaxapi.Result{
					{File: fileA, Text: `"github.com/pkg/errors"`, From: term.Coordinates{X: 1, Y: 5}, To: term.Coordinates{X: 24, Y: 5}, CaptureName: "path"},
				},
				aliases: []syntaxapi.Result{
					{File: fileA, Text: "errs", From: term.Coordinates{X: 1, Y: 5}, To: term.Coordinates{X: 5, Y: 5}, CaptureName: "alias"},
					{File: fileA, Text: `"github.com/pkg/errors"`, From: term.Coordinates{X: 6, Y: 5}, To: term.Coordinates{X: 29, Y: 5}, CaptureName: "path"},
				},
				selectors: []syntaxapi.Result{
					// errs.New — uses the explicit alias
					{File: fileA, Text: "errs", From: term.Coordinates{X: 1, Y: 20}, To: term.Coordinates{X: 5, Y: 20}, CaptureName: "pkg"},
					{File: fileA, Text: "New", From: term.Coordinates{X: 6, Y: 20}, To: term.Coordinates{X: 9, Y: 20}, CaptureName: "symbol"},
					// errors.Wrap — uses old default, no longer valid
					{File: fileA, Text: "errors", From: term.Coordinates{X: 1, Y: 21}, To: term.Coordinates{X: 7, Y: 21}, CaptureName: "pkg"},
					{File: fileA, Text: "Wrap", From: term.Coordinates{X: 8, Y: 21}, To: term.Coordinates{X: 12, Y: 21}, CaptureName: "symbol"},
				},
			},
			want: []string{"errs.New"},
		},
		{
			name: "unexported symbols filtered out",
			results: queryResults{
				imports: []syntaxapi.Result{
					{File: fileA, Text: `"mypkg"`, From: term.Coordinates{X: 1, Y: 5}, To: term.Coordinates{X: 8, Y: 5}, CaptureName: "path"},
				},
				types: []syntaxapi.Result{
					{File: fileA, Text: "mypkg", From: term.Coordinates{X: 5, Y: 10}, To: term.Coordinates{X: 10, Y: 10}, CaptureName: "pkg"},
					{File: fileA, Text: "privateType", From: term.Coordinates{X: 11, Y: 10}, To: term.Coordinates{X: 22, Y: 10}, CaptureName: "type"},
					{File: fileA, Text: "mypkg", From: term.Coordinates{X: 5, Y: 11}, To: term.Coordinates{X: 10, Y: 11}, CaptureName: "pkg"},
					{File: fileA, Text: "Public", From: term.Coordinates{X: 11, Y: 11}, To: term.Coordinates{X: 17, Y: 11}, CaptureName: "type"},
				},
				selectors: []syntaxapi.Result{
					{File: fileA, Text: "mypkg", From: term.Coordinates{X: 1, Y: 20}, To: term.Coordinates{X: 6, Y: 20}, CaptureName: "pkg"},
					{File: fileA, Text: "doStuff", From: term.Coordinates{X: 7, Y: 20}, To: term.Coordinates{X: 14, Y: 20}, CaptureName: "symbol"},
				},
			},
			want: []string{"mypkg.Public"},
		},
		{
			name: "struct field access filtered by import check",
			results: queryResults{
				imports: []syntaxapi.Result{
					{File: fileA, Text: `"fmt"`, From: term.Coordinates{X: 1, Y: 5}, To: term.Coordinates{X: 6, Y: 5}, CaptureName: "path"},
				},
				selectors: []syntaxapi.Result{
					// s.Name — "s" is not an import alias
					{File: fileA, Text: "s", From: term.Coordinates{X: 5, Y: 66}, To: term.Coordinates{X: 6, Y: 66}, CaptureName: "pkg"},
					{File: fileA, Text: "Name", From: term.Coordinates{X: 7, Y: 66}, To: term.Coordinates{X: 11, Y: 66}, CaptureName: "symbol"},
					// cmd.Args — "cmd" is not an import alias
					{File: fileA, Text: "cmd", From: term.Coordinates{X: 8, Y: 161}, To: term.Coordinates{X: 11, Y: 161}, CaptureName: "pkg"},
					{File: fileA, Text: "Args", From: term.Coordinates{X: 12, Y: 161}, To: term.Coordinates{X: 16, Y: 161}, CaptureName: "symbol"},
					// fmt.Println — valid import call
					{File: fileA, Text: "fmt", From: term.Coordinates{X: 1, Y: 170}, To: term.Coordinates{X: 4, Y: 170}, CaptureName: "pkg"},
					{File: fileA, Text: "Println", From: term.Coordinates{X: 5, Y: 170}, To: term.Coordinates{X: 12, Y: 170}, CaptureName: "symbol"},
				},
			},
			want: []string{"fmt.Println"},
		},
		{
			name: "deduplicates across files",
			results: queryResults{
				imports: []syntaxapi.Result{
					{File: fileA, Text: `"fmt"`, From: term.Coordinates{X: 1, Y: 5}, To: term.Coordinates{X: 6, Y: 5}, CaptureName: "path"},
					{File: fileB, Text: `"fmt"`, From: term.Coordinates{X: 1, Y: 5}, To: term.Coordinates{X: 6, Y: 5}, CaptureName: "path"},
				},
				selectors: []syntaxapi.Result{
					{File: fileA, Text: "fmt", From: term.Coordinates{X: 1, Y: 20}, To: term.Coordinates{X: 4, Y: 20}, CaptureName: "pkg"},
					{File: fileA, Text: "Errorf", From: term.Coordinates{X: 5, Y: 20}, To: term.Coordinates{X: 11, Y: 20}, CaptureName: "symbol"},
					{File: fileB, Text: "fmt", From: term.Coordinates{X: 1, Y: 30}, To: term.Coordinates{X: 4, Y: 30}, CaptureName: "pkg"},
					{File: fileB, Text: "Errorf", From: term.Coordinates{X: 5, Y: 30}, To: term.Coordinates{X: 11, Y: 30}, CaptureName: "symbol"},
				},
			},
			want: []string{"fmt.Errorf"},
		},
		{
			name:    "empty workspace",
			results: queryResults{},
			want:    nil,
		},
		{
			name: "dot and blank imports skipped",
			results: queryResults{
				imports: []syntaxapi.Result{
					{File: fileA, Text: `"fmt"`, From: term.Coordinates{X: 1, Y: 5}, To: term.Coordinates{X: 6, Y: 5}, CaptureName: "path"},
				},
				aliases: []syntaxapi.Result{
					{File: fileA, Text: ".", From: term.Coordinates{X: 1, Y: 6}, To: term.Coordinates{X: 2, Y: 6}, CaptureName: "alias"},
					{File: fileA, Text: `"testing"`, From: term.Coordinates{X: 3, Y: 6}, To: term.Coordinates{X: 12, Y: 6}, CaptureName: "path"},
					{File: fileA, Text: "_", From: term.Coordinates{X: 1, Y: 7}, To: term.Coordinates{X: 2, Y: 7}, CaptureName: "alias"},
					{File: fileA, Text: `"embed"`, From: term.Coordinates{X: 3, Y: 7}, To: term.Coordinates{X: 10, Y: 7}, CaptureName: "path"},
				},
				selectors: []syntaxapi.Result{
					{File: fileA, Text: "fmt", From: term.Coordinates{X: 1, Y: 20}, To: term.Coordinates{X: 4, Y: 20}, CaptureName: "pkg"},
					{File: fileA, Text: "Println", From: term.Coordinates{X: 5, Y: 20}, To: term.Coordinates{X: 12, Y: 20}, CaptureName: "symbol"},
				},
			},
			want: []string{"fmt.Println"},
		},
		{
			name: "qualified types not filtered by import check",
			results: queryResults{
				imports: []syntaxapi.Result{
					{File: fileA, Text: `"context"`, From: term.Coordinates{X: 1, Y: 5}, To: term.Coordinates{X: 10, Y: 5}, CaptureName: "path"},
				},
				types: []syntaxapi.Result{
					// Qualified types pass through regardless of import table —
					// tree-sitter already guarantees these are type references.
					{File: fileA, Text: "context", From: term.Coordinates{X: 5, Y: 10}, To: term.Coordinates{X: 12, Y: 10}, CaptureName: "pkg"},
					{File: fileA, Text: "Context", From: term.Coordinates{X: 13, Y: 10}, To: term.Coordinates{X: 20, Y: 10}, CaptureName: "type"},
					// Even if the package name doesn't match an import alias,
					// the qualified type is still valid (could be a type alias, etc.).
					{File: fileA, Text: "io", From: term.Coordinates{X: 5, Y: 11}, To: term.Coordinates{X: 7, Y: 11}, CaptureName: "pkg"},
					{File: fileA, Text: "Reader", From: term.Coordinates{X: 8, Y: 11}, To: term.Coordinates{X: 14, Y: 11}, CaptureName: "type"},
				},
			},
			want: []string{"context.Context", "io.Reader"},
		},
		{
			name: "interleaved results from concurrent files paired correctly",
			results: queryResults{
				imports: []syntaxapi.Result{
					{File: fileA, Text: `"context"`, CaptureName: "path"},
					{File: fileB, Text: `"fmt"`, CaptureName: "path"},
				},
				types: []syntaxapi.Result{
					// Simulate gRPC stream interleaving: pkg from fileA,
					// then pkg from fileB, then type from fileA, then type from fileB.
					{File: fileA, Text: "context", CaptureName: "pkg"},
					{File: fileB, Text: "fmt", CaptureName: "pkg"},
					{File: fileA, Text: "Context", CaptureName: "type"},
					{File: fileB, Text: "Stringer", CaptureName: "type"},
				},
			},
			want: []string{"context.Context", "fmt.Stringer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := &mockParser{searchFn: searchRouter(tt.results)}
			iter, err := completeReferencedSymbol(context.Background(), parser)
			require.NoError(t, err)
			got := collectIter(t, iter)
			assert.Equal(t, tt.want, got)
		})
	}
}


// benchmarkData builds mock parser data at realistic scale.
// Real workspace profile: ~2200 imports, ~170 aliases, ~10K qualified types, ~24K selectors.
func benchmarkData() (parser *mockParser, nUnique int) {
	// Deterministic pseudo-random via simple counter.
	pkgs := []string{"fmt", "context", "strings", "os", "io", "sync", "time", "errors", "path", "sort"}
	sdkPkgs := []string{"semanticapi", "textapi", "workspaceapi", "syntaxapi", "browserapi", "iterator", "component", "term"}
	typeNames := []string{"Context", "Error", "Reader", "Writer", "Buffer", "Handler", "Server", "Client", "Config", "Result"}
	funcNames := []string{"Errorf", "Println", "Sprintf", "TrimPrefix", "HasPrefix", "Join", "Split", "Index", "Replace", "Map"}
	localVars := []string{"s", "m", "r", "p", "cmd", "ctx", "err", "cfg", "buf", "it"}
	fieldNames := []string{"Name", "URI", "File", "Text", "Args", "Type", "Value", "Content", "Location", "Children"}

	const nFiles = 150
	files := make([]workspaceapi.URI, nFiles)
	for i := range files {
		files[i], _ = workspaceapi.ParseURI(fmt.Sprintf("file:///workspace/pkg%d/file%d.go", i/10, i))
	}

	// ~2200 imports: ~15 per file
	var imports []syntaxapi.Result
	for _, f := range files {
		for j, pkg := range pkgs {
			imports = append(imports, syntaxapi.Result{
				File: f, Text: `"` + pkg + `"`,
				From: term.Coordinates{X: 1, Y: 26 + j}, To: term.Coordinates{X: len(pkg) + 3, Y: 26 + j},
				CaptureName: "path",
			})
		}
		for j := 0; j < 5; j++ {
			p := "github.com/unstablebuild/rune-go-sdk/api/" + sdkPkgs[j]
			imports = append(imports, syntaxapi.Result{
				File: f, Text: `"` + p + `"`,
				From: term.Coordinates{X: 1, Y: 36 + j}, To: term.Coordinates{X: len(p) + 3, Y: 36 + j},
				CaptureName: "path",
			})
		}
	}

	// ~170 aliases: ~1 per file
	var aliases []syntaxapi.Result
	for i, f := range files {
		if i%2 == 0 {
			continue
		}
		aliases = append(aliases,
			syntaxapi.Result{File: f, Text: "errs", CaptureName: "alias"},
			syntaxapi.Result{File: f, Text: `"errors"`, CaptureName: "path"},
		)
	}

	// ~10K qualified types: alternating pkg/type
	var types []syntaxapi.Result
	for _, f := range files {
		for _, pkg := range sdkPkgs {
			for _, tn := range typeNames[:7] {
				types = append(types,
					syntaxapi.Result{File: f, Text: pkg, CaptureName: "pkg",
						From: term.Coordinates{X: 5, Y: 50}, To: term.Coordinates{X: 5 + len(pkg), Y: 50}},
					syntaxapi.Result{File: f, Text: tn, CaptureName: "type",
						From: term.Coordinates{X: 6 + len(pkg), Y: 50}, To: term.Coordinates{X: 6 + len(pkg) + len(tn), Y: 50}},
				)
			}
		}
	}

	// ~24K selectors: mix of package calls and struct field access
	var selectors []syntaxapi.Result
	for _, f := range files {
		// Package calls (~60 per file)
		for _, pkg := range pkgs {
			for _, fn := range funcNames[:6] {
				selectors = append(selectors,
					syntaxapi.Result{File: f, Text: pkg, CaptureName: "pkg"},
					syntaxapi.Result{File: f, Text: fn, CaptureName: "symbol"},
				)
			}
		}
		// Struct field access (~100 per file, should be filtered)
		for _, v := range localVars {
			for _, fld := range fieldNames {
				selectors = append(selectors,
					syntaxapi.Result{File: f, Text: v, CaptureName: "pkg"},
					syntaxapi.Result{File: f, Text: fld, CaptureName: "symbol"},
				)
			}
		}
	}

	router := func(query string, _ []string) (iterator.Iterator[syntaxapi.Result], error) {
		switch {
		case strings.Contains(query, "import_spec") && !strings.Contains(query, "name:"):
			return iterator.FromSlice(imports), nil
		case strings.Contains(query, "import_spec") && strings.Contains(query, "name:"):
			return iterator.FromSlice(aliases), nil
		case strings.Contains(query, "qualified_type"):
			return iterator.FromSlice(types), nil
		case strings.Contains(query, "selector_expression"):
			return iterator.FromSlice(selectors), nil
		}
		return iterator.Empty[syntaxapi.Result](), nil
	}

	// Count expected unique results for sanity check.
	seen := make(map[string]bool)
	for i := 0; i < len(types); i += 2 {
		s := types[i].Text + "." + types[i+1].Text
		if isExported(types[i+1].Text) {
			seen[s] = true
		}
	}
	for i := 0; i < len(selectors); i += 2 {
		pkg, sym := selectors[i].Text, selectors[i+1].Text
		if isExported(sym) {
			for _, p := range pkgs {
				if pkg == p {
					seen[pkg+"."+sym] = true
					break
				}
			}
		}
	}

	return &mockParser{searchFn: router}, len(seen)
}

func BenchmarkCompleteReferencedSymbol(b *testing.B) {
	parser, expectedUnique := benchmarkData()
	ctx := context.Background()

	// Sanity check on first run.
	iter, err := completeReferencedSymbol(ctx, parser)
	if err != nil {
		b.Fatal(err)
	}
	n := 0
	for {
		_, ok := iter.Next(ctx)
		if !ok {
			break
		}
		n++
	}
	if err := iter.Close(); err != nil {
		b.Fatal(err)
	}
	b.Logf("unique results: %d (expected ~%d)", n, expectedUnique)

	b.ResetTimer()
	for range b.N {
		iter, _ := completeReferencedSymbol(ctx, parser)
		for {
			_, ok := iter.Next(ctx)
			if !ok {
				break
			}
		}
		_ = iter.Close()
	}
}
