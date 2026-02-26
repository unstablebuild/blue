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

var _ textapi.CommandHandler = (*implementationHandler)(nil)

func TestImplementationHandler(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///project")
	require.NoError(t, err)

	tests := []struct {
		name        string
		result      semanticapi.LocationResult
		nilResource bool
		wantFloat   bool
		wantEntries int
	}{
		{
			name: "single implementation",
			result: semanticapi.LocationResult{
				Location: &semanticapi.Location{
					URI: "file:///project/a.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 15, Character: 0},
						End:   semanticapi.Position{Line: 15, Character: 5},
					},
				},
			},
			wantFloat:   true,
			wantEntries: 1,
		},
		{
			name: "multiple implementations",
			result: semanticapi.LocationResult{
				Locations: []semanticapi.Location{
					{URI: "file:///project/a.go", Range: semanticapi.Range{Start: semanticapi.Position{Line: 10}}},
					{URI: "file:///project/b.go", Range: semanticapi.Range{Start: semanticapi.Position{Line: 20}}},
				},
			},
			wantFloat:   true,
			wantEntries: 2,
		},
		{name: "no implementations", wantFloat: false},
		{name: "nil resource", nilResource: true, wantFloat: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lsp := &mockLSP{
				implementationFn: func(_ context.Context, _ semanticapi.ImplementationParams) (semanticapi.LocationResult, error) {
					return tt.result, nil
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
			h := ImplementationHandler(
				lsp, editor, wm, &mockResourceOpener{}, &mockNotifications{}, &mockFileSystem{},
				rootURI, syncTick, nil, DefaultImplementationConfig(),
			)

			uri, _ := workspaceapi.ParseURI("file:///project/a.go")
			cmd := textapi.Command{Name: "implementation", URI: uri}
			if !tt.nilResource {
				cmd.Resource = &mockHandler{uri: uri}
			}
			cmd.Cursor.Content = term.Coordinates{X: 5, Y: 50}

			err := h.HandleCommand(context.Background(), cmd)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFloat, fh != nil)
			if tt.wantEntries > 0 {
				lh := fh.(*locationsFloatingHandler)
				assert.Equal(t, tt.wantEntries, len(lh.entries))
			}
		})
	}
}
