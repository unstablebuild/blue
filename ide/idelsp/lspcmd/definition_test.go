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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

var _ textapi.CommandHandler = (*definitionHandler)(nil)

func TestDefinitionHandler(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///project")
	require.NoError(t, err)

	fileB, err := workspaceapi.ParseURI("file:///project/b.go")
	require.NoError(t, err)

	tests := []struct {
		name         string
		result       semanticapi.LocationResult
		nilResource  bool
		args         []string
		parser       *mockParser
		wantErr      bool
		wantFloat    bool
		wantNavigate bool
		wantEntries  int
	}{
		{
			name: "single definition navigates directly",
			result: semanticapi.LocationResult{
				Location: &semanticapi.Location{
					URI: "file:///project/a.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 15, Character: 0},
						End:   semanticapi.Position{Line: 15, Character: 5},
					},
				},
			},
			wantNavigate: true,
		},
		{
			name: "multiple definitions",
			result: semanticapi.LocationResult{
				Locations: []semanticapi.Location{
					{URI: "file:///project/a.go", Range: semanticapi.Range{Start: semanticapi.Position{Line: 10}}},
					{URI: "file:///project/b.go", Range: semanticapi.Range{Start: semanticapi.Position{Line: 20}}},
				},
			},
			wantFloat:   true,
			wantEntries: 2,
		},
		{name: "no definitions"},
		{name: "nil resource", nilResource: true, wantErr: true},
		{
			name:        "definition via symbol name",
			nilResource: true,
			args:        []string{"mylib.MyFunc"},
			parser: &mockParser{
				searchFn: func(query string, _ []string) (iterator.Iterator[syntaxapi.Result], error) {
					if strings.Contains(query, "selector_expression") {
						return iterator.FromSlice([]syntaxapi.Result{
							{File: fileB, Text: "mylib", From: term.Coordinates{X: 1, Y: 42}, CaptureName: "pkg"},
							{File: fileB, Text: "MyFunc", From: term.Coordinates{X: 7, Y: 42}, CaptureName: "symbol"},
						}), nil
					}
					return iterator.Empty[syntaxapi.Result](), nil
				},
			},
			result: semanticapi.LocationResult{
				Location: &semanticapi.Location{
					URI: "file:///project/b.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 42, Character: 7},
						End:   semanticapi.Position{Line: 42, Character: 13},
					},
				},
			},
			wantNavigate: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lsp := &mockLSP{
				definitionFn: func(_ context.Context, _ semanticapi.DefinitionParams) (semanticapi.LocationResult, error) {
					return tt.result, nil
				},
			}
			done := make(chan struct{}, 1)
			var navigated bool
			editor := &mockEditor{
				editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
					return &mockHandler{uri: u}, nil
				},
				setCursorFn: func(_ textapi.Handler, _ term.Coordinates) error {
					navigated = true
					select {
					case done <- struct{}{}:
					default:
					}
					return nil
				},
			}
			var fh browserapi.Floating
			wm := &mockWindowManager{
				floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
					fh = h
					select {
					case done <- struct{}{}:
					default:
					}
					return nil, nil
				},
			}
			h := DefinitionHandler(
				lsp, editor, wm, &mockResourceOpener{}, &mockNotifications{}, &mockFileSystem{},
				syncTick, tt.parser, DefinitionConfig{RootURI: rootURI}, nil,
			)

			uri, _ := workspaceapi.ParseURI("file:///project/a.go")
			cmd := textapi.Command{Name: "definition", URI: uri, Args: tt.args}
			if !tt.nilResource {
				cmd.Resource = &mockHandler{uri: uri}
			}
			cmd.Cursor.Content = term.Coordinates{X: 5, Y: 50}

		err := h.HandleCommand(context.Background(), cmd)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if len(tt.args) > 0 && (tt.wantNavigate || tt.wantFloat) {
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for async resolution")
				}
			}
			assert.Equal(t, tt.wantFloat, fh != nil)
			assert.Equal(t, tt.wantNavigate, navigated)
			if tt.wantEntries > 0 {
				lh := fh.(*locationsFloatingHandler)
				assert.Equal(t, tt.wantEntries, len(lh.entries))
			}
		})
	}
}
