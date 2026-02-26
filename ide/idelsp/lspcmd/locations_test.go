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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestNavigateTo(t *testing.T) {
	tests := []struct {
		name    string
		entry   locationEntry
		wantURI string
		wantPos term.Coordinates
	}{
		{
			name: "navigates to position",
			entry: locationEntry{
				uri: "file:///foo/bar.go",
				rng: semanticapi.Range{
					Start: semanticapi.Position{
						Line:      10,
						Character: 5,
					},
				},
			},
			wantURI: "/foo/bar.go",
			wantPos: term.Coordinates{
				X: 5, Y: 10,
			},
		},
		{
			name: "line zero character zero",
			entry: locationEntry{
				uri: "file:///root.go",
				rng: semanticapi.Range{
					Start: semanticapi.Position{},
				},
			},
			wantURI: "/root.go",
			wantPos: term.Coordinates{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				openedPath       string
				cursorSet        term.Coordinates
				setContentWindow browserapi.Window
				setContentH      browserapi.Handler
			)
			openHandler := &mockHandler{}
			opener := &mockResourceOpener{
				openFn: func(
					u workspaceapi.URI,
				) (browserapi.Handler, error) {
					openedPath = u.Path()
					return openHandler, nil
				},
			}
			focusWindow := &mockWindow{id: 1}
			wm := &mockWindowManager{
				focusFn: func() (browserapi.Window, error) {
					return focusWindow, nil
				},
				setWindowContentFn: func(w browserapi.Window, h browserapi.Handler) error {
					setContentWindow = w
					setContentH = h
					return nil
				},
			}
			editor := &mockEditor{
				editorFn: func(
					u workspaceapi.URI,
				) (textapi.Handler, error) {
					return &mockHandler{
						uri: u,
					}, nil
				},
				setCursorFn: func(
					_ textapi.Handler,
					c term.Coordinates,
				) error {
					cursorSet = c
					return nil
				},
			}
			syncTick := func(fn func()) bool { fn(); return true }
			navigateTo(
				tt.entry, opener, wm, editor, &mockNotifications{}, syncTick,
			)
			assert.Equal(
				t, tt.wantURI, openedPath,
			)
			assert.Equal(t, focusWindow, setContentWindow)
			assert.Same(t, openHandler, setContentH, "SetWindowContent should receive the handler from Open")
			assert.Equal(
				t, tt.wantPos, cursorSet,
			)
		})
	}
}

func TestLocationsFromResult(t *testing.T) {
	tests := []struct {
		name   string
		result semanticapi.LocationResult
		want   []locationEntry
	}{
		{
			name: "single location",
			result: semanticapi.LocationResult{
				Location: &semanticapi.Location{
					URI: "file:///a.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{
							Line: 9,
						},
					},
				},
			},
			want: []locationEntry{
				{
					uri: "file:///a.go",
					rng: semanticapi.Range{
						Start: semanticapi.Position{
							Line: 9,
						},
					},
					display: "/a.go:10",
				},
			},
		},
		{
			name: "multiple locations",
			result: semanticapi.LocationResult{
				Locations: []semanticapi.Location{
					{
						URI: "file:///a.go",
						Range: semanticapi.Range{
							Start: semanticapi.Position{
								Line: 1,
							},
						},
					},
					{
						URI: "file:///b.go",
						Range: semanticapi.Range{
							Start: semanticapi.Position{
								Line: 2,
							},
						},
					},
				},
			},
			want: []locationEntry{
				{
					uri: "file:///a.go",
					rng: semanticapi.Range{
						Start: semanticapi.Position{
							Line: 1,
						},
					},
					display: "/a.go:2",
				},
				{
					uri: "file:///b.go",
					rng: semanticapi.Range{
						Start: semanticapi.Position{
							Line: 2,
						},
					},
					display: "/b.go:3",
				},
			},
		},
		{
			name: "location links",
			result: semanticapi.LocationResult{
				LocationLinks: []semanticapi.LocationLink{
					{
						TargetURI: "file:///c.go",
						TargetSelectionRange: semanticapi.Range{
							Start: semanticapi.Position{
								Line: 4,
							},
						},
					},
				},
			},
			want: []locationEntry{
				{
					uri: "file:///c.go",
					rng: semanticapi.Range{
						Start: semanticapi.Position{
							Line: 4,
						},
					},
					display: "/c.go:5",
				},
			},
		},
		{
			name:   "empty result",
			result: semanticapi.LocationResult{},
		},
		{
			name: "location display line+1",
			result: semanticapi.LocationResult{
				Location: &semanticapi.Location{
					URI: "file:///src/main.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{
							Line: 41,
						},
					},
				},
			},
			want: []locationEntry{
				{
					uri: "file:///src/main.go",
					rng: semanticapi.Range{
						Start: semanticapi.Position{
							Line: 41,
						},
					},
					display: "/src/main.go:42",
				},
			},
		},
		{
			name: "link display line+1",
			result: semanticapi.LocationResult{
				LocationLinks: []semanticapi.LocationLink{
					{
						TargetURI: "file:///lib.go",
						TargetSelectionRange: semanticapi.Range{
							Start: semanticapi.Position{
								Line: 0,
							},
						},
					},
				},
			},
			want: []locationEntry{
				{
					uri:     "file:///lib.go",
					rng:     semanticapi.Range{},
					display: "/lib.go:1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := locationsFromResult(
				tt.result,
			)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEnrichEntriesRelativePaths(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///workspace")
	require.NoError(t, err)

	entries := []locationEntry{
		{
			uri: "file:///workspace/pkg/foo.go",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 5}},
		},
		{
			uri: "file:///other/bar.go",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 0}},
		},
	}
	got := enrichEntries(entries, rootURI)

	assert.Equal(t, "pkg/foo.go:6", got[0].display, "workspace file should be relative")
	assert.Equal(t, "/other/bar.go:1", got[1].display, "out-of-workspace file should be absolute")
}

func TestTrimFilePrefix(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "with prefix",
			uri:  "file:///foo/bar.go",
			want: "/foo/bar.go",
		},
		{
			name: "without prefix",
			uri:  "/foo/bar.go",
			want: "/foo/bar.go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimFilePrefix(tt.uri)
			assert.Equal(t, tt.want, got)
		})
	}
}
