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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

func TestResolveSymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		query       string
		symbols     []semanticapi.SymbolInformation
		lspErr      error
		wantMatches []SymbolMatch
		wantErr     bool
	}{
		{
			name:  "exact match",
			query: "MyFunc",
			symbols: []semanticapi.SymbolInformation{
				{
					Name: "MyFuncHelper",
					Location: semanticapi.Location{
						URI:   "file:///project/helper.go",
						Range: semanticapi.Range{Start: semanticapi.Position{Line: 5, Character: 0}},
					},
				},
				{
					Name: "MyFunc",
					Location: semanticapi.Location{
						URI:   "file:///project/main.go",
						Range: semanticapi.Range{Start: semanticapi.Position{Line: 10, Character: 5}},
					},
				},
			},
			wantMatches: []SymbolMatch{{
				URI:     "file:///project/main.go",
				Pos:     semanticapi.Position{Line: 10, Character: 5},
				Display: "MyFunc",
			}},
		},
		{
			name:  "fuzzy fallback",
			query: "MyFunc",
			symbols: []semanticapi.SymbolInformation{
				{
					Name: "MyFuncHelper",
					Location: semanticapi.Location{
						URI:   "file:///project/helper.go",
						Range: semanticapi.Range{Start: semanticapi.Position{Line: 3, Character: 0}},
					},
				},
			},
			wantMatches: []SymbolMatch{{
				URI:     "file:///project/helper.go",
				Pos:     semanticapi.Position{Line: 3, Character: 0},
				Display: "MyFuncHelper",
			}},
		},
		{
			name:    "no results",
			query:   "Missing",
			symbols: nil,
			wantErr: true,
		},
		{
			name:    "lsp error",
			query:   "Err",
			lspErr:  errors.New("workspace symbol failed"),
			wantErr: true,
		},
		{
			name:  "multiple exact matches different packages",
			query: "Buffer",
			symbols: []semanticapi.SymbolInformation{
				{
					Name: "Buffer",
					Location: semanticapi.Location{
						URI:   "file:///project/bytes/buffer.go",
						Range: semanticapi.Range{Start: semanticapi.Position{Line: 10}},
					},
				},
				{
					Name: "Buffer",
					Location: semanticapi.Location{
						URI:   "file:///project/cell/buffer.go",
						Range: semanticapi.Range{Start: semanticapi.Position{Line: 20}},
					},
				},
			},
			wantMatches: []SymbolMatch{
				{URI: "file:///project/bytes/buffer.go", Pos: semanticapi.Position{Line: 10}, Display: "bytes.Buffer"},
				{URI: "file:///project/cell/buffer.go", Pos: semanticapi.Position{Line: 20}, Display: "cell.Buffer"},
			},
		},
		{
			name:  "multiple exact matches same short package disambiguated",
			query: "Buffer",
			symbols: []semanticapi.SymbolInformation{
				{
					Name: "Buffer",
					Location: semanticapi.Location{
						URI:   "file:///project/pkgA/cell/buffer.go",
						Range: semanticapi.Range{Start: semanticapi.Position{Line: 10}},
					},
				},
				{
					Name: "Buffer",
					Location: semanticapi.Location{
						URI:   "file:///project/pkgB/cell/buffer.go",
						Range: semanticapi.Range{Start: semanticapi.Position{Line: 20}},
					},
				},
			},
			wantMatches: []SymbolMatch{
				{URI: "file:///project/pkgA/cell/buffer.go", Pos: semanticapi.Position{Line: 10}, Display: "/project/pkgA/cell.Buffer"},
				{URI: "file:///project/pkgB/cell/buffer.go", Pos: semanticapi.Position{Line: 20}, Display: "/project/pkgB/cell.Buffer"},
			},
		},
		{
			name:  "duplicate URIs deduplicated",
			query: "Buffer",
			symbols: []semanticapi.SymbolInformation{
				{
					Name: "Buffer",
					Location: semanticapi.Location{
						URI:   "file:///project/cell/buffer.go",
						Range: semanticapi.Range{Start: semanticapi.Position{Line: 10}},
					},
				},
				{
					Name: "Buffer",
					Location: semanticapi.Location{
						URI:   "file:///project/cell/buffer.go",
						Range: semanticapi.Range{Start: semanticapi.Position{Line: 10}},
					},
				},
			},
			wantMatches: []SymbolMatch{{
				URI:     "file:///project/cell/buffer.go",
				Pos:     semanticapi.Position{Line: 10},
				Display: "Buffer",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lsp := &mockLSP{
				workspaceSymbolFn: func(_ context.Context, _ semanticapi.WorkspaceSymbolParams) ([]semanticapi.SymbolInformation, error) {
					return tt.symbols, tt.lspErr
				},
			}
			matches, err := ResolveSymbol(context.Background(), lsp, tt.query)
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

func TestCompleteSymbol(t *testing.T) {
	t.Parallel()

	t.Run("empty query returns all symbol names", func(t *testing.T) {
		t.Parallel()
		lsp := &mockLSP{
			workspaceSymbolFn: func(_ context.Context, p semanticapi.WorkspaceSymbolParams) ([]semanticapi.SymbolInformation, error) {
				assert.Empty(t, p.Query)
				return []semanticapi.SymbolInformation{
					{Name: "Alpha"},
					{Name: "Beta"},
					{Name: "Gamma"},
				}, nil
			},
		}
		iter, err := CompleteSymbol(context.Background(), lsp, "")
		require.NoError(t, err)

		var names []string
		for {
			v, ok := iter.Next(context.Background())
			if !ok {
				break
			}
			names = append(names, v)
		}
		assert.Equal(t, []string{"Alpha", "Beta", "Gamma"}, names)
	})

	t.Run("non-empty query is forwarded", func(t *testing.T) {
		t.Parallel()
		lsp := &mockLSP{
			workspaceSymbolFn: func(_ context.Context, p semanticapi.WorkspaceSymbolParams) ([]semanticapi.SymbolInformation, error) {
				assert.Equal(t, "My", p.Query)
				return []semanticapi.SymbolInformation{
					{Name: "MyFunc"},
					{Name: "MyType"},
				}, nil
			},
		}
		iter, err := CompleteSymbol(context.Background(), lsp, "My")
		require.NoError(t, err)

		var names []string
		for {
			v, ok := iter.Next(context.Background())
			if !ok {
				break
			}
			names = append(names, v)
		}
		assert.Equal(t, []string{"MyFunc", "MyType"}, names)
	})

	t.Run("lsp error propagated", func(t *testing.T) {
		t.Parallel()
		lsp := &mockLSP{
			workspaceSymbolFn: func(_ context.Context, _ semanticapi.WorkspaceSymbolParams) ([]semanticapi.SymbolInformation, error) {
				return nil, errors.New("fail")
			},
		}
		_, err := CompleteSymbol(context.Background(), lsp, "")
		require.Error(t, err)
	})
}
