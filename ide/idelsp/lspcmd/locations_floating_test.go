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
	"errors"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/iterator"
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

var syncTick = func(fn func()) bool { fn(); return true }

func testFloatingHandler(entries []locationEntry, fc fileContent) *locationsFloatingHandler {
	cfg := LocationsConfig{PreviewAttr: term.Attributes{Bg: tcell.ColorYellow}}
	return newLocationsFloatingHandler(
		entries, &mockResourceOpener{}, &mockWindowManager{}, &mockEditor{},
		&mockNotifications{}, testFS(fc), syncTick, nil, cfg,
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
		{
			name:        "ctrl-j handled",
			event:       term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'j'},
			wantHandled: true,
		},
		{
			name:        "ctrl-k handled",
			event:       term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'k'},
			wantHandled: true,
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
	leftPad := spanHPad / 2 // Span centres content; left offset = hPad/2
	// Target line (line 1) should have preview bg.
	cell := rendered[1*w+leftPad]
	assert.Equal(t, tcell.ColorYellow, cell.Bg, "target line should have preview bg")
	// Non-target line should not.
	assert.NotEqual(t, tcell.ColorYellow, rendered[leftPad].Bg, "non-target line should not have preview bg")
}

func TestLocationsFloatingDrawPreviewReverseRange(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:1 Foo",
			rng: semanticapi.Range{
				Start: semanticapi.Position{Line: 0, Character: 2},
				End:   semanticapi.Position{Line: 0, Character: 5},
			},
		},
	}
	fc := fileContent{"/a.go": "0123456789"}
	h := testFloatingHandler(entries, fc)
	w, ht := h.Dimensions()
	h.Resize(w, ht)
	sw := term.NewStringWriter(w, ht)
	h.Draw(sw)
	require.NoError(t, sw.Flush())

	rendered := sw.Cells()
	leftPad := spanHPad / 2 // Span centres content; left offset = hPad/2
	// Character 1 is outside the range — no AttrReverse.
	assert.Zero(t, rendered[leftPad+1].Attrs&tcell.AttrReverse, "char before range should not have AttrReverse")
	// Character 2 is inside the range — AttrReverse set.
	assert.NotZero(t, rendered[leftPad+2].Attrs&tcell.AttrReverse, "char in range should have AttrReverse")
	// Character 4 is inside the range.
	assert.NotZero(t, rendered[leftPad+4].Attrs&tcell.AttrReverse, "char in range should have AttrReverse")
	// Character 5 is outside the range (end is exclusive).
	assert.Zero(t, rendered[leftPad+5].Attrs&tcell.AttrReverse, "char after range should not have AttrReverse")
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
	leftPad := spanHPad / 2 // Span centres content; left offset = hPad/2
	// The last line of the file should get the target highlight.
	cell := rendered[1*w+leftPad]
	assert.Equal(t, tcell.ColorYellow, cell.Bg, "clamped target line should have preview bg")
}

func TestLocationsFloatingHandlerDimensions(t *testing.T) {
	entries := []locationEntry{{display: "a.go:1"}}
	h := newLocationsFloatingHandler(entries, &mockResourceOpener{}, &mockWindowManager{}, &mockEditor{}, &mockNotifications{}, &mockFileSystem{}, syncTick, nil, LocationsConfig{})
	w, ht := h.Dimensions()
	assert.Equal(t, minPreviewWidth+spanHPad, w)
	assert.Equal(t, 1+previewContextLines+separatorHeight+spanVPad, ht)
}

func TestLocationsFloatingHandlerDimensionsWideEntries(t *testing.T) {
	// An entry wider than minPreviewWidth should push the ideal width.
	long := strings.Repeat("x", minPreviewWidth+20)
	entries := []locationEntry{{display: long}}
	h := newLocationsFloatingHandler(entries, &mockResourceOpener{}, &mockWindowManager{}, &mockEditor{}, &mockNotifications{}, &mockFileSystem{}, syncTick, nil, LocationsConfig{})
	w, ht := h.Dimensions()
	assert.Equal(t, utf8.RuneCountInString(long)+spanHPad, w)
	assert.Equal(t, 1+previewContextLines+separatorHeight+spanVPad, ht)
}

func TestLocationsFloatingResize(t *testing.T) {
	entries := []locationEntry{{display: "a.go:1"}}
	h := newLocationsFloatingHandler(entries, &mockResourceOpener{}, &mockWindowManager{}, &mockEditor{}, &mockNotifications{}, &mockFileSystem{}, syncTick, nil, LocationsConfig{})

	// Dimensions returns ideal size (inner + padding).
	w, ht := h.Dimensions()
	assert.Equal(t, minPreviewWidth+spanHPad, w)
	assert.Equal(t, 1+previewContextLines+separatorHeight+spanVPad, ht)

	// Resize with smaller values; inner dimensions exclude padding.
	h.Resize(30, 10)
	assert.Equal(t, 30-spanHPad, h.innerW)
	assert.Equal(t, 10-spanVPad, h.innerH)

	// Dimensions still returns ideal.
	w, ht = h.Dimensions()
	assert.Equal(t, minPreviewWidth+spanHPad, w)
	assert.Equal(t, 1+previewContextLines+separatorHeight+spanVPad, ht)
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
		entries, opener, &mockWindowManager{}, &mockEditor{}, notify, &mockFileSystem{}, syncTick, nil, LocationsConfig{},
	)
	exit, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, exit)
	assert.True(t, handled)
	assert.True(t, notified, "should notify on navigation error")
}

// mockParser implements syntaxapi.Parser for testing.
type mockParser struct {
	highlightFn func(workspaceapi.URI, string) (iterator.Iterator[textapi.Location], error)
	searchFn    func(string, []string) (iterator.Iterator[syntaxapi.Result], error)
}

func (m *mockParser) Search(query string, captures []string, langs ...string) (iterator.Iterator[syntaxapi.Result], error) {
	if m.searchFn != nil {
		return m.searchFn(query, captures)
	}
	return iterator.Empty[syntaxapi.Result](), nil
}
func (m *mockParser) SearchNode(_ syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}
func (m *mockParser) Query(_ workspaceapi.URI, _ string, _ []string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}
func (m *mockParser) QueryNode(_ workspaceapi.URI, _ syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}
func (m *mockParser) Highlight(uri workspaceapi.URI, content string) (iterator.Iterator[textapi.Location], error) {
	if m.highlightFn != nil {
		return m.highlightFn(uri, content)
	}
	return iterator.Empty[textapi.Location](), nil
}

func TestLocationsFloatingHighlightApplied(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:2 Foo",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 1}},
		},
	}
	fc := fileContent{"/a.go": "func main() {}\nfoo bar baz"}
	h := testFloatingHandler(entries, fc)
	// Bake highlight attrs directly into the cells.
	for x := 0; x < 4 && x < len(h.previewCells[0]); x++ {
		h.previewCells[0][x].Attributes = term.Attributes{Fg: tcell.ColorGreen}
	}

	w, ht := h.Dimensions()
	h.Resize(w, ht)
	sw := term.NewStringWriter(w, ht)
	h.Draw(sw)
	require.NoError(t, sw.Flush())

	rendered := sw.Cells()
	leftPad := spanHPad / 2 // Span centres content; left offset = hPad/2
	// Line 0 / char 0 is within the highlight range — should have green fg.
	assert.Equal(t, tcell.ColorGreen, rendered[leftPad+0].Fg, "highlighted char should have green fg")
	// Line 0 / char 5 is outside the range — should not have green fg.
	assert.NotEqual(t, tcell.ColorGreen, rendered[leftPad+5].Fg, "non-highlighted char should not have green fg")
}

func TestLocationsFloatingHighlightUnionWithTargetLine(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:1 Foo",
			rng: semanticapi.Range{
				Start: semanticapi.Position{Line: 0, Character: 1},
				End:   semanticapi.Position{Line: 0, Character: 3},
			},
		},
	}
	fc := fileContent{"/a.go": "func main"}
	h := testFloatingHandler(entries, fc)
	// Bake highlight attrs directly into the cells.
	for x := 0; x < 4 && x < len(h.previewCells[0]); x++ {
		h.previewCells[0][x].Attributes = term.Attributes{Fg: tcell.ColorRed}
	}

	w, ht := h.Dimensions()
	h.Resize(w, ht)
	sw := term.NewStringWriter(w, ht)
	h.Draw(sw)
	require.NoError(t, sw.Flush())

	rendered := sw.Cells()
	leftPad := spanHPad / 2 // Span centres content; left offset = hPad/2
	// Char 0: target line, within highlight range, but outside reference range.
	// Should have the highlight fg (Red) unioned with the preview attr (Yellow bg).
	cell0 := rendered[leftPad+0]
	assert.Equal(t, tcell.ColorYellow, cell0.Bg, "target line cell should have preview bg")
	assert.Equal(t, tcell.ColorRed, cell0.Fg, "target line cell should keep syntax fg")
	assert.Zero(t, cell0.Attrs&tcell.AttrReverse, "cell outside reference range should not have reverse")

	// Char 2: target line, within highlight range AND within reference range.
	cell2 := rendered[leftPad+2]
	assert.Equal(t, tcell.ColorYellow, cell2.Bg, "ref range cell should have preview bg")
	assert.Equal(t, tcell.ColorRed, cell2.Fg, "ref range cell should keep syntax fg")
	assert.NotZero(t, cell2.Attrs&tcell.AttrReverse, "ref range cell should have reverse")
}

func TestLocationsFloatingHighlightMultipleRangesPerLine(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:2 x",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 1}},
		},
	}
	fc := fileContent{"/a.go": "func main\nreturn"}
	h := testFloatingHandler(entries, fc)
	// Bake highlight attrs directly into the cells.
	for x := 0; x < 4 && x < len(h.previewCells[0]); x++ {
		h.previewCells[0][x].Attributes = term.Attributes{Fg: tcell.ColorBlue}
	}
	for x := 5; x < 9 && x < len(h.previewCells[0]); x++ {
		h.previewCells[0][x].Attributes = term.Attributes{Fg: tcell.ColorRed}
	}

	w, ht := h.Dimensions()
	h.Resize(w, ht)
	sw := term.NewStringWriter(w, ht)
	h.Draw(sw)
	require.NoError(t, sw.Flush())

	rendered := sw.Cells()
	leftPad := spanHPad / 2 // Span centres content; left offset = hPad/2
	assert.Equal(t, tcell.ColorBlue, rendered[leftPad+0].Fg, "first range should be blue")
	assert.Equal(t, tcell.ColorRed, rendered[leftPad+5].Fg, "second range should be red")
}

func TestLocationsFloatingNilParserNoHighlights(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:1 Foo",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 0}},
		},
	}
	fc := fileContent{"/a.go": "line0\nline1"}
	h := testFloatingHandler(entries, fc)

	// With nil parser, cells have no syntax attrs — just verify content renders.
	require.NotNil(t, h.previewCells, "previewCells should be set from file content")

	w, ht := h.Dimensions()
	h.Resize(w, ht)
	sw := term.NewStringWriter(w, ht)
	h.Draw(sw)
	require.NoError(t, sw.Flush())
	assert.Contains(t, sw.String(), "line0")
}

func TestLocationsFloatingLoadHighlights(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:1 Foo",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 0}},
		},
	}
	fc := fileContent{"/a.go": "line0\nline1\nline2"}
	h := testFloatingHandler(entries, fc)

	parser := &mockParser{
		highlightFn: func(_ workspaceapi.URI, _ string) (iterator.Iterator[textapi.Location], error) {
			return iterator.FromSlice([]textapi.Location{
				{
					From: term.Coordinates{X: 0, Y: 0},
					To:   term.Coordinates{X: 5, Y: 0},
					Attr: term.Attributes{Fg: tcell.ColorGreen},
				},
				{
					From: term.Coordinates{X: 0, Y: 1},
					To:   term.Coordinates{X: 5, Y: 1},
					Attr: term.Attributes{Fg: tcell.ColorBlue},
				},
			}), nil
		},
	}
	h.parser = parser
	content := "line0\nline1\nline2"
	baseCells := term.CloneCells(h.previewCells)
	uri, _ := workspaceapi.ParseURI("file:///a.go")
	h.loadHighlights(uri, content, baseCells)

	require.NotNil(t, h.previewCells)
	require.Len(t, h.previewCells, 3)
	// Line 0: all 5 chars should have green fg.
	for x := 0; x < 5; x++ {
		assert.Equal(t, tcell.ColorGreen, h.previewCells[0][x].Fg, "line 0 char %d", x)
	}
	// Line 1: all 5 chars should have blue fg.
	for x := 0; x < 5; x++ {
		assert.Equal(t, tcell.ColorBlue, h.previewCells[1][x].Fg, "line 1 char %d", x)
	}
	// Line 2: no highlights — attrs should be default.
	for _, c := range h.previewCells[2] {
		assert.Equal(t, tcell.ColorDefault, c.Fg, "line 2 should have default fg")
	}
}

func TestLocationsFloatingLoadHighlightsErrorIgnored(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:1 Foo",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 0}},
		},
	}
	fc := fileContent{"/a.go": "line0"}
	h := testFloatingHandler(entries, fc)

	// Snapshot the cells before calling loadHighlights with a failing parser.
	before := term.CloneCells(h.previewCells)

	parser := &mockParser{
		highlightFn: func(_ workspaceapi.URI, _ string) (iterator.Iterator[textapi.Location], error) {
			return nil, errors.New("highlight unavailable")
		},
	}
	h.parser = parser
	baseCells := term.CloneCells(h.previewCells)
	uri, _ := workspaceapi.ParseURI("file:///a.go")
	h.loadHighlights(uri, "line0", baseCells)

	// Cells should be unchanged when parser returns error.
	assert.Equal(t, before, h.previewCells, "previewCells should be unchanged when parser returns error")
}

func TestLocationsFloatingLoadHighlightsOutOfBoundsIgnored(t *testing.T) {
	entries := []locationEntry{
		{
			uri: "file:///a.go", display: "a.go:1 Foo",
			rng: semanticapi.Range{Start: semanticapi.Position{Line: 0}},
		},
	}
	fc := fileContent{"/a.go": "line0"}
	h := testFloatingHandler(entries, fc)

	// Snapshot the cells before calling loadHighlights with out-of-bounds data.
	before := term.CloneCells(h.previewCells)

	parser := &mockParser{
		highlightFn: func(_ workspaceapi.URI, _ string) (iterator.Iterator[textapi.Location], error) {
			return iterator.FromSlice([]textapi.Location{
				{
					From: term.Coordinates{X: 0, Y: 99},
					To:   term.Coordinates{X: 5, Y: 99},
					Attr: term.Attributes{Fg: tcell.ColorGreen},
				},
			}), nil
		},
	}
	h.parser = parser
	baseCells := term.CloneCells(h.previewCells)
	uri, _ := workspaceapi.ParseURI("file:///a.go")
	h.loadHighlights(uri, "line0", baseCells)

	// Cell attrs should be unchanged — out-of-bounds highlights are skipped.
	require.NotNil(t, h.previewCells)
	for i, row := range h.previewCells {
		for j, c := range row {
			assert.Equal(t, before[i][j].Attributes, c.Attributes,
				"cell [%d][%d] attrs should be unchanged for out-of-bounds highlight", i, j)
		}
	}
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
