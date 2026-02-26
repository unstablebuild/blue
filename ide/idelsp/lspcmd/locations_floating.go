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
	"io"
	"log/slog"
	"os"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

const (
	previewContextLines = 21 // +/- 10 lines around the reference
	minPreviewWidth     = 80
	maxListHeight       = 15
	separatorHeight     = 1
	spanHPad            = 2
	spanVPad            = 0
)

// LocationsConfig configures the appearance of a locations floating handler.
type LocationsConfig struct {
	ListTextAttr  term.Attributes
	ListFocusAttr term.Attributes
	PreviewAttr   term.Attributes
}

// DefaultLocationsConfig returns a LocationsConfig with sensible defaults.
func DefaultLocationsConfig() LocationsConfig {
	return LocationsConfig{
		ListFocusAttr: term.Attributes{Fg: tcell.ColorPurple, Attrs: tcell.AttrBold},
	}
}

type locationsFloatingHandler struct {
	entries          []locationEntry
	list             *component.FocusList
	editor           textapi.Editor
	opener           browserapi.ResourceOpener
	wm               browserapi.WindowManager
	notify           browserapi.Notifications
	fs               workspaceapi.FileSystem
	scheduleNextTick func(func()) bool
	parser           syntaxapi.Parser

	previewCells [][]term.Cell // cell matrix for the currently previewed file
	prevURI      string        // URI of the currently loaded preview

	maxEntryW int // widest entry display in rune count
	innerW    int // inner content width from Resize (excludes padding)
	innerH    int // inner content height from Resize (excludes padding)
	previewH  int // computed preview height from Resize
	listH     int // computed list height from Resize

	span        *component.Span
	previewAttr term.Attributes
}

// locationsInner adapts the handler's inner drawing logic (preview + separator + list)
// as a tui.Component so that component.Span can manage padding and alignment.
type locationsInner struct {
	handler *locationsFloatingHandler
}

func (li *locationsInner) Dimensions() (int, int) {
	h := li.handler
	idealW := h.maxEntryW
	for _, cells := range h.previewCells {
		if l := len(cells); l > idealW {
			idealW = l
		}
	}
	if idealW < minPreviewWidth {
		idealW = minPreviewWidth
	}
	listH := len(h.entries)
	if listH > maxListHeight {
		listH = maxListHeight
	}
	return idealW, listH + previewContextLines + separatorHeight
}

func (li *locationsInner) Resize(w, h int) {
	handler := li.handler
	handler.innerW = w
	handler.innerH = h
	handler.previewH = previewContextLines
	if handler.previewH > h-1-separatorHeight {
		handler.previewH = h - 1 - separatorHeight
	}
	if handler.previewH < 0 {
		handler.previewH = 0
	}
	handler.listH = h - handler.previewH - separatorHeight
	if handler.listH < 1 {
		handler.listH = 1
	}
	handler.list.Resize(w, handler.listH)
}

func (li *locationsInner) Draw(w term.Writer) {
	h := li.handler
	h.drawPreview(w)
	h.drawSeparator(w)
	vw := &component.VirtualWriter{
		Writer: w,
		Offset: term.Coordinates{Y: h.previewH + separatorHeight},
		Width:  h.innerW,
		Height: h.listH,
	}
	h.list.Draw(vw)
}

func newLocationsFloatingHandler(
	entries []locationEntry,
	opener browserapi.ResourceOpener, wm browserapi.WindowManager,
	editor textapi.Editor, notify browserapi.Notifications,
	fs workspaceapi.FileSystem, scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser, cfg LocationsConfig,
) *locationsFloatingHandler {
	list := &component.FocusList{}
	list.InitWithAttr(cfg.ListTextAttr, cfg.ListFocusAttr)
	maxEntryW := 0
	for _, e := range entries {
		list.PushBack(component.NewResponsiveString(e.display, component.StringResponsiveConfig{}))
		if w := utf8.RuneCountInString(e.display); w > maxEntryW {
			maxEntryW = w
		}
	}
	handler := &locationsFloatingHandler{
		entries:          entries,
		list:             list,
		editor:           editor,
		opener:           opener,
		wm:               wm,
		notify:           notify,
		fs:               fs,
		scheduleNextTick: scheduleNextTick,
		parser:           parser,
		maxEntryW:        maxEntryW,
		previewAttr:      cfg.PreviewAttr,
	}
	handler.span = component.NewSpan(&locationsInner{handler}, component.SpanConfig{
		PadHorizontal:    spanHPad,
		PadVertical:      spanVPad,
		ContentAlignment: component.AlignmentCentered,
	})
	handler.loadPreview()
	return handler
}

func (l *locationsFloatingHandler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return false, false
	}
	switch ev.Key {
	case term.KeyEsc:
		return true, true
	case term.KeyEnter:
		idx := l.list.FocusOffset()
		if idx < len(l.entries) {
			navigateTo(l.entries[idx], l.opener, l.wm, l.editor, l.notify, l.scheduleNextTick)
		}
		return true, true
	case term.KeyArrowUp:
		l.list.FocusUp()
		l.loadPreview()
		return false, true
	case term.KeyArrowDown:
		l.list.FocusDown()
		l.loadPreview()
		return false, true
	}
	if ev.Mod == term.ModCtrl {
		switch ev.Ch {
		case 'j':
			l.list.FocusDown()
			l.loadPreview()
			return false, true
		case 'k':
			l.list.FocusUp()
			l.loadPreview()
			return false, true
		}
	}
	return false, false
}

func (l *locationsFloatingHandler) Draw(w term.Writer) {
	l.span.Draw(w)
}

func (l *locationsFloatingHandler) drawPreview(w term.Writer) {
	idx := l.list.FocusOffset()
	if idx >= len(l.entries) || l.previewCells == nil {
		return
	}
	entry := l.entries[idx]
	targetLine := int(entry.rng.Start.Line)
	if targetLine >= len(l.previewCells) {
		targetLine = len(l.previewCells) - 1
		if targetLine < 0 {
			targetLine = 0
		}
	}
	startLine := targetLine - l.previewH/2
	if startLine < 0 {
		startLine = 0
	}
	if startLine+l.previewH > len(l.previewCells) {
		startLine = len(l.previewCells) - l.previewH
		if startLine < 0 {
			startLine = 0
		}
	}
	startChar := int(entry.rng.Start.Character)
	endChar := int(entry.rng.End.Character)
	for row := range l.previewH {
		srcLine := startLine + row
		if srcLine >= len(l.previewCells) {
			break
		}
		isTarget := srcLine == targetLine
		cells := l.previewCells[srcLine]
		for x := range l.innerW {
			var cell term.Cell
			if x < len(cells) {
				cell = cells[x]
			} else {
				cell = term.Cell{Ch: ' ', Width: 1}
			}
			if isTarget {
				cell.Attributes = term.AttributesUnion(cell.Attributes, l.previewAttr)
				if x >= startChar && x < endChar {
					cell.Attrs |= tcell.AttrReverse
				}
			}
			w.SetCell(term.Coordinates{X: x, Y: row}, cell)
		}
	}
}

func (l *locationsFloatingHandler) drawSeparator(w term.Writer) {
	ch := component.FrameCharSetDefault().HorizontalTop
	attr := term.Attributes{Fg: tcell.ColorGray}
	y := l.previewH
	for x := range l.innerW {
		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{Ch: ch, Width: 1, Attributes: attr})
	}
}

func (l *locationsFloatingHandler) Dimensions() (int, int) {
	return l.span.Dimensions()
}

func (l *locationsFloatingHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

func (l *locationsFloatingHandler) Selection() (string, bool) {
	idx := l.list.FocusOffset()
	if idx >= len(l.entries) {
		return "", false
	}
	return l.entries[idx].display, true
}

func (l *locationsFloatingHandler) Resize(w, h int) {
	l.span.Resize(w, h)
}

func (l *locationsFloatingHandler) Close() error { return nil }

func (l *locationsFloatingHandler) loadPreview() {
	idx := l.list.FocusOffset()
	if idx >= len(l.entries) {
		return
	}
	entry := l.entries[idx]
	if entry.uri == l.prevURI {
		return
	}
	uri, err := lspToURI(entry.uri)
	if err != nil {
		l.previewCells = nil
		l.prevURI = ""
		return
	}
	f, err := l.fs.OpenFile(uri.Path(), os.O_RDONLY, 0)
	if err != nil {
		l.previewCells = nil
		l.prevURI = ""
		return
	}
	defer f.Close() //nolint:errcheck
	data, err := io.ReadAll(f)
	if err != nil {
		l.previewCells = nil
		l.prevURI = ""
		return
	}
	content := string(data)
	l.previewCells = term.StringToCells(content)
	l.prevURI = entry.uri
	if l.parser != nil {
		baseCells := term.CloneCells(l.previewCells)
		fileURI := uri
		go l.loadHighlights(fileURI, content, baseCells)
	}
}

func (l *locationsFloatingHandler) loadHighlights(
	uri workspaceapi.URI, content string, baseCells [][]term.Cell,
) {
	iter, err := l.parser.Highlight(uri, content)
	if err != nil {
		return
	}
	defer func() { _ = iter.Close() }()
	highlighted := term.CloneCells(baseCells)
	for {
		loc, ok := iter.Next(context.Background())
		if !ok {
			break
		}
		y := loc.From.Y
		if y < 0 || y >= len(highlighted) {
			continue
		}
		row := highlighted[y]
		for x := loc.From.X; x < loc.To.X && x < len(row); x++ {
			row[x].Attributes = loc.Attr
		}
	}
	if err := iter.Err(); err != nil {
		slog.Warn("highlight iteration", "uri", uri, "err", err)
		return
	}
	l.scheduleNextTick(func() {
		l.previewCells = highlighted
	})
}
