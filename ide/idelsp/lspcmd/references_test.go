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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

var _ textapi.CommandHandler = (*referencesHandler)(nil)

func TestReferencesEnrichedDisplay(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///project")
	require.NoError(t, err)

	locs := []semanticapi.Location{
		{
			URI: "file:///project/a.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 0, Character: 5},
				End:   semanticapi.Position{Line: 0, Character: 8},
			},
		},
		{
			URI: "file:///project/b.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 3, Character: 0},
				End:   semanticapi.Position{Line: 3, Character: 3},
			},
		},
	}
	lsp := &mockLSP{
		referencesFn: func(_ context.Context, _ semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
			return locs, nil
		},
	}
	editor := &mockEditor{
		editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: u}, nil
		},
	}
	var fh browserapi.Floating
	wm := &mockWindowManager{
		floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
			fh = h
			return nil, nil
		},
	}
	h := ReferencesHandler(
		lsp, editor, wm, &mockResourceOpener{}, &mockNotifications{}, &mockFileSystem{},
		rootURI, syncTick, nil, DefaultReferencesConfig(), nil,
	)

	uri, _ := workspaceapi.ParseURI("file:///project/a.go")
	cmd := textapi.Command{Name: "references", URI: uri, Resource: &mockHandler{uri: uri}}
	cmd.Cursor.Content = term.Coordinates{X: 5, Y: 0}

	err = h.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, fh)

	lh := fh.(*locationsFloatingHandler)
	assert.Equal(t, "a.go:1", lh.entries[0].display)
}

func TestReferencesRelativePaths(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///workspace")
	require.NoError(t, err)

	locs := []semanticapi.Location{
		{
			URI: "file:///workspace/src/pkg/handler.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 10, Character: 5},
				End:   semanticapi.Position{Line: 10, Character: 11},
			},
		},
		{
			URI: "file:///workspace/cmd/main.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 3, Character: 0},
				End:   semanticapi.Position{Line: 3, Character: 6},
			},
		},
		{
			URI: "file:///other/lib/ext.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 0, Character: 0},
				End:   semanticapi.Position{Line: 0, Character: 3},
			},
		},
	}
	lsp := &mockLSP{
		referencesFn: func(_ context.Context, _ semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
			return locs, nil
		},
	}
	editor := &mockEditor{
		editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: u}, nil
		},
	}
	var fh browserapi.Floating
	wm := &mockWindowManager{
		floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
			fh = h
			return nil, nil
		},
	}
	h := ReferencesHandler(
		lsp, editor, wm, &mockResourceOpener{}, &mockNotifications{}, &mockFileSystem{},
		rootURI, syncTick, nil, ReferencesConfig{ListConfig: DefaultLocationsConfig()}, nil,
	)

	uri, _ := workspaceapi.ParseURI("file:///workspace/src/pkg/handler.go")
	cmd := textapi.Command{Name: "references", URI: uri, Resource: &mockHandler{uri: uri}}
	cmd.Cursor.Content = term.Coordinates{X: 5, Y: 10}

	err = h.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, fh)

	lh := fh.(*locationsFloatingHandler)
	require.Len(t, lh.entries, 3)
	assert.Equal(t, "src/pkg/handler.go:11", lh.entries[0].display, "workspace nested path should be relative")
	assert.Equal(t, "cmd/main.go:4", lh.entries[1].display, "workspace path should be relative")
	assert.Equal(t, "/other/lib/ext.go:1", lh.entries[2].display, "out-of-workspace path should be absolute")
}

func TestReferencesZeroRootURI(t *testing.T) {
	locs := []semanticapi.Location{
		{
			URI: "file:///workspace/pkg/foo.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 5, Character: 0},
				End:   semanticapi.Position{Line: 5, Character: 3},
			},
		},
		{
			URI: "file:///workspace/pkg/bar.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 10, Character: 0},
				End:   semanticapi.Position{Line: 10, Character: 3},
			},
		},
	}
	lsp := &mockLSP{
		referencesFn: func(_ context.Context, _ semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
			return locs, nil
		},
	}
	editor := &mockEditor{
		editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: u}, nil
		},
	}
	var fh browserapi.Floating
	wm := &mockWindowManager{
		floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
			fh = h
			return nil, nil
		},
	}
	// Zero RootURI (not set) — paths should fall back to absolute.
	h := ReferencesHandler(
		lsp, editor, wm, &mockResourceOpener{}, &mockNotifications{}, &mockFileSystem{},
		workspaceapi.URI{}, syncTick, nil, DefaultReferencesConfig(), nil,
	)

	uri, _ := workspaceapi.ParseURI("file:///workspace/pkg/foo.go")
	cmd := textapi.Command{Name: "references", URI: uri, Resource: &mockHandler{uri: uri}}
	cmd.Cursor.Content = term.Coordinates{X: 0, Y: 5}

	err := h.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, fh)

	lh := fh.(*locationsFloatingHandler)
	require.Len(t, lh.entries, 2)
	assert.Equal(t, "/workspace/pkg/foo.go:6", lh.entries[0].display, "zero RootURI should produce absolute path")
	assert.Equal(t, "/workspace/pkg/bar.go:11", lh.entries[1].display, "zero RootURI should produce absolute path")
}

func TestReferencesHandler(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///project")
	require.NoError(t, err)

	tests := []struct {
		name         string
		locs         []semanticapi.Location
		nilResource  bool
		wantErr      bool
		wantFloat    bool
		wantNavigate bool
		wantEntries  int
	}{
		{
			name: "single reference navigates directly",
			locs: []semanticapi.Location{
				{
					URI: "file:///project/a.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 10, Character: 5},
						End:   semanticapi.Position{Line: 10, Character: 8},
					},
				},
			},
			wantNavigate: true,
		},
		{
			name: "multiple references",
			locs: []semanticapi.Location{
				{
					URI: "file:///project/a.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 10, Character: 5},
						End:   semanticapi.Position{Line: 10, Character: 8},
					},
				},
				{
					URI: "file:///project/b.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 20, Character: 0},
						End:   semanticapi.Position{Line: 20, Character: 3},
					},
				},
			},
			wantFloat:   true,
			wantEntries: 2,
		},
		{name: "zero references"},
		{name: "nil resource", nilResource: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lsp := &mockLSP{
				referencesFn: func(_ context.Context, _ semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
					return tt.locs, nil
				},
			}
			var navigated bool
			editor := &mockEditor{
				editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
					return &mockHandler{uri: u}, nil
				},
				setCursorFn: func(_ textapi.Handler, _ term.Coordinates) error {
					navigated = true
					return nil
				},
			}
			var fh browserapi.Floating
			wm := &mockWindowManager{
				floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
					fh = h
					return nil, nil
				},
			}
			h := ReferencesHandler(
				lsp, editor, wm, &mockResourceOpener{}, &mockNotifications{}, &mockFileSystem{},
				rootURI, syncTick, nil, DefaultReferencesConfig(), nil,
			)

			uri, _ := workspaceapi.ParseURI("file:///project/a.go")
			cmd := textapi.Command{Name: "references", URI: uri}
			if !tt.nilResource {
				cmd.Resource = &mockHandler{uri: uri}
			}
			cmd.Cursor.Content = term.Coordinates{X: 5, Y: 10}

		err := h.HandleCommand(context.Background(), cmd)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantFloat, fh != nil)
			assert.Equal(t, tt.wantNavigate, navigated)
			if tt.wantEntries > 0 {
				lh := fh.(*locationsFloatingHandler)
				assert.Equal(t, tt.wantEntries, len(lh.entries))
			}
		})
	}
}
