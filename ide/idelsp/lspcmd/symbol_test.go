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
		name    string
		query   string
		symbols []semanticapi.SymbolInformation
		lspErr  error
		wantURI string
		wantPos semanticapi.Position
		wantErr bool
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
			wantURI: "file:///project/main.go",
			wantPos: semanticapi.Position{Line: 10, Character: 5},
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
			wantURI: "file:///project/helper.go",
			wantPos: semanticapi.Position{Line: 3, Character: 0},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lsp := &mockLSP{
				workspaceSymbolFn: func(_ context.Context, _ semanticapi.WorkspaceSymbolParams) ([]semanticapi.SymbolInformation, error) {
					return tt.symbols, tt.lspErr
				},
			}
			uri, pos, err := ResolveSymbol(context.Background(), lsp, tt.query)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURI, uri)
			assert.Equal(t, tt.wantPos, pos)
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
