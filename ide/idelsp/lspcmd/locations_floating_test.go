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
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

var _ browserapi.Floating = (*locationsFloatingHandler)(nil)

// fileContent maps file paths to their text content for the mock FS.
type fileContent map[string]string

func testFS(content fileContent) *mockFileSystem {
	return &mockFileSystem{
		openFileFn: func(path string, _ int, _ os.FileMode) (workspaceapi.File, error) {
			text, ok := content[path]
			if !ok {
				return nil, os.ErrNotExist
			}
			return newStringFile(text), nil
		},
	}
}

func testFloatingHandler(entries []locationEntry, fc fileContent) *locationsFloatingHandler {
	cfg := LocationsConfig{PreviewAttr: term.Attributes{Bg: tcell.ColorYellow}}
	return newLocationsFloatingHandler(
		entries, &mockResourceOpener{}, &mockWindowManager{}, &mockEditor{},
		&mockNotifications{}, testFS(fc), cfg,
	)
}

func snapshot(h *locationsFloatingHandler) string {
	w, ht := h.Dimensions()
	h.Resize(w, ht)
	sw := term.NewStringWriter(w, ht)
	h.Draw(sw)
	_ = sw.Flush()
	return sw.String()
}

func TestLocationsFloatingHandlerRender(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:1 Foo",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 0}},
		},
		{
			uri: "file:///a.go", display: "a.go:3 Bar",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 2}},
		},
	}
	fc := fileContent{"/a.go": "line0\nline1\nline2"}
	h := testFloatingHandler(entries, fc)
	expected := snapshot(h)
	w, ht := h.Dimensions()

	cases := []handlertest.SequenceTestCase{
		{InputSequence: "", Expected: expected},
	}
	handlertest.RunHandlerSequence(t, h, w, ht, cases)
}

func TestLocationsFloatingNavPreview(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:1 Foo",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 0}},
		},
		{
			uri: "file:///b.go", display: "b.go:2 Bar",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 1}},
		},
	}
	fc := fileContent{
		"/a.go": "aline0\naline1\naline2",
		"/b.go": "bline0\nbline1\nbline2",
	}
	h := testFloatingHandler(entries, fc)
	w, ht := h.Dimensions()
	h.Resize(w, ht)

	sw := term.NewStringWriter(w, h.previewH)
	h.drawPreview(sw)
	require.NoError(t, sw.Flush())
	assert.Contains(t, sw.String(), "aline0", "initial preview should show a.go")

	// After down: preview loads b.go content via FileSystem.
	h.list.FocusDown()
	h.loadPreview()
	sw = term.NewStringWriter(w, h.previewH)
	h.drawPreview(sw)
	require.NoError(t, sw.Flush())
	assert.Contains(t, sw.String(), "bline0", "after down preview should show b.go")
}

func TestLocationsFloatingHandlerHandle(t *testing.T) {
	tests := []struct {
		name        string
		event       term.Event
		wantExit    bool
		wantHandled bool
	}{
		{
			name:        "esc exits",
			event:       term.Event{Type: term.EventKey, Key: term.KeyEsc},
			wantExit:    true,
			wantHandled: true,
		},
		{
			name:        "enter exits",
			event:       term.Event{Type: term.EventKey, Key: term.KeyEnter},
			wantExit:    true,
			wantHandled: true,
		},
		{
			name:        "up handled",
			event:       term.Event{Type: term.EventKey, Key: term.KeyArrowUp},
			wantHandled: true,
		},
		{
			name:        "down handled",
			event:       term.Event{Type: term.EventKey, Key: term.KeyArrowDown},
			wantHandled: true,
		},
		{
			name:  "non-key ignored",
			event: term.Event{Type: term.EventMouse},
		},
		{
			name:  "unknown key ignored",
			event: term.Event{Type: term.EventKey, Key: term.KeyTab},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []locationEntry{{display: "a.go:1"}, {display: "b.go:2"}}
			h := testFloatingHandler(entries, nil)
			exit, handled := h.Handle(tt.event)
			assert.Equal(t, tt.wantExit, exit)
			assert.Equal(t, tt.wantHandled, handled)
		})
	}
}

func TestLocationsFloatingHandlerDraw(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:2 Foo",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 1}},
		},
	}
	fc := fileContent{"/a.go": "line0\nline1\nline2"}
	h := testFloatingHandler(entries, fc)
	w, ht := h.Dimensions()
	h.Resize(w, ht)
	sw := term.NewStringWriter(w, ht)
	h.Draw(sw)
	require.NoError(t, sw.Flush())

	rendered := sw.Cells()
	// Target line (line 1) should have preview bg.
	cell := rendered[1*w]
	assert.Equal(t, tcell.ColorYellow, cell.Bg, "target line should have preview bg")
	// Non-target line should not.
	assert.NotEqual(t, tcell.ColorYellow, rendered[0].Bg, "non-target line should not have preview bg")
}

func TestLocationsFloatingDrawPreviewClampsTargetLine(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:11 Oob",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 10}},
		},
	}
	fc := fileContent{"/a.go": "line0\nline1"}
	h := testFloatingHandler(entries, fc)
	w, ht := h.Dimensions()
	h.Resize(w, ht)
	sw := term.NewStringWriter(w, ht)
	h.Draw(sw)
	require.NoError(t, sw.Flush())

	rendered := sw.Cells()
	// The last line of the file should get the target highlight.
	cell := rendered[1*w]
	assert.Equal(t, tcell.ColorYellow, cell.Bg, "clamped target line should have preview bg")
}

func TestLocationsFloatingHandlerDimensions(t *testing.T) {
	entries := []locationEntry{{display: "a.go:1"}}
	h := newLocationsFloatingHandler(entries, &mockResourceOpener{}, &mockWindowManager{}, &mockEditor{}, &mockNotifications{}, &mockFileSystem{}, LocationsConfig{})
	w, ht := h.Dimensions()
	assert.Equal(t, minPreviewWidth, w)
	assert.Equal(t, 1+previewContextLines, ht)
}

func TestLocationsFloatingHandlerDimensionsWideEntries(t *testing.T) {
	// An entry wider than minPreviewWidth should push the ideal width.
	long := strings.Repeat("x", minPreviewWidth+20)
	entries := []locationEntry{{display: long}}
	h := newLocationsFloatingHandler(entries, &mockResourceOpener{}, &mockWindowManager{}, &mockEditor{}, &mockNotifications{}, &mockFileSystem{}, LocationsConfig{})
	w, ht := h.Dimensions()
	assert.Equal(t, utf8.RuneCountInString(long), w)
	assert.Equal(t, 1+previewContextLines, ht)
}

func TestLocationsFloatingResize(t *testing.T) {
	entries := []locationEntry{{display: "a.go:1"}}
	h := newLocationsFloatingHandler(entries, &mockResourceOpener{}, &mockWindowManager{}, &mockEditor{}, &mockNotifications{}, &mockFileSystem{}, LocationsConfig{})

	// Dimensions returns ideal size.
	w, ht := h.Dimensions()
	assert.Equal(t, minPreviewWidth, w)
	assert.Equal(t, 1+previewContextLines, ht)

	// Resize with smaller values.
	h.Resize(30, 10)
	assert.Equal(t, 30, h.actualW)
	assert.Equal(t, 10, h.actualH)

	// Dimensions still returns ideal.
	w, ht = h.Dimensions()
	assert.Equal(t, minPreviewWidth, w)
	assert.Equal(t, 1+previewContextLines, ht)
}

func TestLocationsFloatingCursorSelection(t *testing.T) {
	entries := []locationEntry{{display: "a.go:1 Foo"}}
	h := testFloatingHandler(entries, nil)

	_, _, visible := h.Cursor()
	assert.False(t, visible)

	sel, hasSel := h.Selection()
	assert.True(t, hasSel)
	assert.Equal(t, "a.go:1 Foo", sel)
}

func TestLocationsFloatingEnterNotifiesOnError(t *testing.T) {
	entries := []locationEntry{
		{uri: "file:///nonexistent.go", display: "x.go:1"},
	}
	var notified bool
	notify := &mockNotifications{
		notifyFn: func(level browserapi.NotificationLevel, _ string, _ ...any) (string, error) {
			if level == browserapi.LevelError {
				notified = true
			}
			return "", nil
		},
	}
	opener := &mockResourceOpener{
		openFn: func(_ workspaceapi.URI) (browserapi.Handler, error) {
			return nil, os.ErrNotExist
		},
	}
	h := newLocationsFloatingHandler(
		entries, opener, &mockWindowManager{}, &mockEditor{}, notify, &mockFileSystem{}, LocationsConfig{},
	)
	exit, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, exit)
	assert.True(t, handled)
	assert.True(t, notified, "should notify on navigation error")
}

// stringFile implements workspaceapi.File backed by an in-memory string.
type stringFile struct{ *strings.Reader }

func newStringFile(s string) *stringFile         { return &stringFile{strings.NewReader(s)} }
func (f *stringFile) Close() error               { return nil }
func (f *stringFile) Name() string               { return "" }
func (f *stringFile) Stat() (os.FileInfo, error) { return nil, nil }
func (f *stringFile) Sync() error                { return nil }
func (f *stringFile) Truncate(_ int64) error     { return nil }
func (f *stringFile) Fd() uintptr                { return 0 }
func (f *stringFile) Write(_ []byte) (int, error) {
	return 0, os.ErrPermission
}
func (f *stringFile) WriteAt(_ []byte, _ int64) (int, error) {
	return 0, os.ErrPermission
}
