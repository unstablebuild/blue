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
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// TestLocationsHandlerRender uses handlertest to
// verify that text content is rendered correctly
// and remains stable across key navigation events.
// Color differences between selected/unselected
// entries are tested separately in
// TestLocationsHandlerDraw.
func TestLocationsHandlerRender(t *testing.T) {
	entries := []locationEntry{
		{display: "a.go:1"},
		{display: "b.go:2"},
	}
	lh := newLocationsHandler(entries, nil, nil, nil)
	w, h := lh.Dimensions()
	expected := "a.go:1  \nb.go:2  "
	cases := []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected:      expected,
		},
		{
			InputSequence: "<down>",
			Expected:      expected,
		},
		{
			InputSequence: "<up>",
			Expected:      expected,
		},
	}
	handlertest.RunHandlerSequence(
		t, lh, w, h, cases,
	)
}

func TestLocationsHandlerHandle(t *testing.T) {
	tests := []struct {
		name         string
		entries      int
		initial      int
		event        term.Event
		wantExit     bool
		wantHandled  bool
		wantSelected int
	}{
		{
			name:    "esc exits",
			entries: 2,
			event: term.Event{
				Type: term.EventKey,
				Key:  term.KeyEsc,
			},
			wantExit:    true,
			wantHandled: true,
		},
		{
			name:    "enter exits",
			entries: 2,
			event: term.Event{
				Type: term.EventKey,
				Key:  term.KeyEnter,
			},
			wantExit:    true,
			wantHandled: true,
		},
		{
			name:    "down moves selection",
			entries: 3,
			event: term.Event{
				Type: term.EventKey,
				Key:  term.KeyArrowDown,
			},
			wantHandled:  true,
			wantSelected: 1,
		},
		{
			name:    "up moves selection",
			entries: 3,
			initial: 1,
			event: term.Event{
				Type: term.EventKey,
				Key:  term.KeyArrowUp,
			},
			wantHandled: true,
		},
		{
			name:    "down at bottom stays",
			entries: 2,
			initial: 1,
			event: term.Event{
				Type: term.EventKey,
				Key:  term.KeyArrowDown,
			},
			wantHandled:  true,
			wantSelected: 1,
		},
		{
			name:    "up at top stays",
			entries: 2,
			event: term.Event{
				Type: term.EventKey,
				Key:  term.KeyArrowUp,
			},
			wantHandled: true,
		},
		{
			name:    "non-key event ignored",
			entries: 2,
			event: term.Event{
				Type: term.EventMouse,
			},
		},
		{
			name:    "unknown key not handled",
			entries: 2,
			event: term.Event{
				Type: term.EventKey,
				Key:  term.KeyTab,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := make(
				[]locationEntry, tt.entries,
			)
			for i := range entries {
				entries[i] = locationEntry{
					display: "x",
				}
			}
			lh := newLocationsHandler(
				entries, nil, nil, nil,
			)
			lh.selected = tt.initial
			exit, handled := lh.Handle(tt.event)
			assert.Equal(
				t, tt.wantExit, exit,
			)
			assert.Equal(
				t, tt.wantHandled, handled,
			)
			assert.Equal(
				t, tt.wantSelected,
				lh.selected,
			)
		})
	}
}

func TestLocationsHandlerDraw(t *testing.T) {
	tests := []struct {
		name     string
		entries  []locationEntry
		selected int
		wantFg   []tcell.Color
	}{
		{
			name: "first selected",
			entries: []locationEntry{
				{display: "a.go:1"},
				{display: "b.go:2"},
			},
			selected: 0,
			wantFg: []tcell.Color{
				tcell.ColorWhite,
				tcell.ColorGray,
			},
		},
		{
			name: "second selected",
			entries: []locationEntry{
				{display: "a.go:1"},
				{display: "b.go:2"},
			},
			selected: 1,
			wantFg: []tcell.Color{
				tcell.ColorGray,
				tcell.ColorWhite,
			},
		},
		{
			name: "single entry selected",
			entries: []locationEntry{
				{display: "only.go:1"},
			},
			wantFg: []tcell.Color{
				tcell.ColorWhite,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lh := newLocationsHandler(
				tt.entries, nil, nil, nil,
			)
			lh.selected = tt.selected
			w, h := lh.Dimensions()
			sw := term.NewStringWriter(w, h)
			lh.Draw(sw)
			require.NoError(t, sw.Flush())

			cells := sw.Cells()
			for i, want := range tt.wantFg {
				cell := cells[i*w]
				assert.Equal(
					t, want, cell.Fg,
					"entry %d foreground", i,
				)
			}
		})
	}
}

func TestLocationsHandlerDimensions(
	t *testing.T,
) {
	tests := []struct {
		name    string
		entries []locationEntry
		wantW   int
		wantH   int
	}{
		{
			name: "width is max display plus 2",
			entries: []locationEntry{
				{display: "short"},
				{display: "longer entry"},
			},
			wantW: utf8.RuneCountInString("longer entry") + 2,
			wantH: 2,
		},
		{
			name: "height capped at 15",
			entries: make(
				[]locationEntry, 20,
			),
			wantW: 2,
			wantH: 15,
		},
		{
			name: "single entry",
			entries: []locationEntry{
				{display: "a.go:1"},
			},
			wantW: 8,
			wantH: 1,
		},
		{
			name: "unicode display uses rune count",
			entries: []locationEntry{
				{display: "café.go:1"},
			},
			wantW: 11,
			wantH: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lh := newLocationsHandler(
				tt.entries, nil, nil, nil,
			)
			w, h := lh.Dimensions()
			assert.Equal(t, tt.wantW, w)
			assert.Equal(t, tt.wantH, h)
		})
	}
}

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
				openedPath string
				cursorSet  term.Coordinates
			)
			opener := &mockResourceOpener{
				openFn: func(
					u workspaceapi.URI,
				) (browserapi.Handler, error) {
					openedPath = u.Path()
					return &mockHandler{}, nil
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
			wm := &mockWindowManager{}
			err := navigateTo(
				tt.entry, opener, wm, editor,
			)
			require.NoError(t, err)
			assert.Equal(
				t, tt.wantURI, openedPath,
			)
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
