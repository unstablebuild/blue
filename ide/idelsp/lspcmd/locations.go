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
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

var _ browserapi.Floating = (*locationsHandler)(nil)

type locationEntry struct {
	uri     string
	rng     semanticapi.Range
	display string
}

// TODO: add a preview of the file at the selected position.
// TODO: make entries relative to workspace URI if possible.
// TODO: scroll list down when cursor passes last visible line.
// TODO: consider using a FocusList instead of reimplementing.
// TODO: allow ctrl-j/k for moving up and down.
type locationsHandler struct {
	entries  []locationEntry
	selected int
	opener   browserapi.ResourceOpener
	wm       browserapi.WindowManager
	editor   textapi.Editor
	width    int
	height   int
}

func newLocationsHandler(
	entries []locationEntry,
	opener browserapi.ResourceOpener,
	wm browserapi.WindowManager,
	editor textapi.Editor,
) *locationsHandler {
	maxW := 0
	for _, e := range entries {
		if n := utf8.RuneCountInString(e.display); n > maxW {
			maxW = n
		}
	}
	h := len(entries)
	if h > 15 {
		h = 15
	}
	return &locationsHandler{
		entries: entries,
		opener:  opener,
		wm:      wm,
		editor:  editor,
		width:   maxW + 2,
		height:  h,
	}
}

func (l *locationsHandler) Handle(
	ev term.Event,
) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return false, false
	}
	switch ev.Key {
	case term.KeyEsc:
		return true, true
	case term.KeyEnter:
		if err := l.navigate(); err != nil {
			slog.Warn("location navigate", "err", err)
		}
		return true, true
	case term.KeyArrowUp:
		if l.selected > 0 {
			l.selected--
		}
		return false, true
	case term.KeyArrowDown:
		if l.selected < len(l.entries)-1 {
			l.selected++
		}
		return false, true
	}
	return false, false
}

func (l *locationsHandler) Cursor() (
	term.Coordinates, term.CursorStyle, bool,
) {
	return term.Coordinates{},
		term.CursorStyleDefault, false
}

func (l *locationsHandler) Selection() (
	string, bool,
) {
	return "", false
}

func (l *locationsHandler) Resize(_, _ int) {}

func (l *locationsHandler) Draw(
	w term.Writer,
) {
	selAttr := term.Attributes{
		Fg: tcell.ColorWhite,
	}
	normAttr := term.Attributes{
		Fg: tcell.ColorGray,
	}
	for i, e := range l.entries {
		if i >= l.height {
			break
		}
		attr := normAttr
		if i == l.selected {
			attr = selAttr
		}
		drawLine(w, i, e.display, l.width, attr)
	}
}

func (l *locationsHandler) Dimensions() (
	int, int,
) {
	return l.width, l.height
}

func (l *locationsHandler) Close() error {
	return nil
}

func (l *locationsHandler) navigate() error {
	if l.selected >= len(l.entries) {
		return nil
	}
	return navigateTo(
		l.entries[l.selected], l.opener, l.wm, l.editor,
	)
}

func navigateTo(
	e locationEntry, opener browserapi.ResourceOpener,
	wm browserapi.WindowManager, editor textapi.Editor,
) error {
	uri, err := lspToURI(e.uri)
	if err != nil {
		return err
	}
	h, err := opener.Open(uri)
	if err != nil {
		return err
	}
	focus, err := wm.Focus()
	if err != nil {
		return err
	}
	err = wm.SetWindowContent(focus, h)
	if err != nil && !errors.Is(err, browserapi.ErrTabNotFree) {
		return err
	}
	eh, err := editor.Editor(uri)
	if err != nil {
		return err
	}
	return editor.SetCursor(eh, posToCoord(e.rng.Start))
}

func locationsFromResult(
	r semanticapi.LocationResult,
) []locationEntry {
	var entries []locationEntry
	if r.Location != nil {
		entries = append(
			entries,
			locationFromLoc(*r.Location),
		)
	}
	for _, loc := range r.Locations {
		entries = append(
			entries, locationFromLoc(loc),
		)
	}
	for _, ll := range r.LocationLinks {
		entries = append(
			entries, locationEntry{
				uri: ll.TargetURI,
				rng: ll.TargetSelectionRange,
				display: fmt.Sprintf(
					"%s:%d",
					trimFilePrefix(ll.TargetURI),
					ll.TargetSelectionRange.
						Start.Line+1,
				),
			},
		)
	}
	return entries
}

func locationFromLoc(
	loc semanticapi.Location,
) locationEntry {
	return locationEntry{
		uri: loc.URI,
		rng: loc.Range,
		display: fmt.Sprintf(
			"%s:%d",
			trimFilePrefix(loc.URI),
			loc.Range.Start.Line+1,
		),
	}
}

func trimFilePrefix(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}

func enrichEntries(
	entries []locationEntry, rootURI workspaceapi.URI, editor textapi.Editor,
) []locationEntry {
	cache := make(map[string][][]term.Cell)
	for i, e := range entries {
		cells, ok := cache[e.uri]
		if !ok {
			cells = loadCells(e.uri, editor)
			if cells != nil {
				cache[e.uri] = cells
			}
		}
		rel := relativePath(e.uri, rootURI)
		if cells != nil {
			sym := extractSymbolName(cells, e.rng)
			entries[i].display = fmt.Sprintf("%s:%d %s", rel, e.rng.Start.Line+1, sym)
		} else {
			entries[i].display = fmt.Sprintf("%s:%d", rel, e.rng.Start.Line+1)
		}
	}
	return entries
}

func loadCells(
	lspURI string, editor textapi.Editor,
) [][]term.Cell {
	uri, err := lspToURI(lspURI)
	if err != nil {
		return nil
	}
	eh, err := editor.Editor(uri)
	if err != nil {
		return nil
	}
	cv := editor.CellView(eh)
	if cv == nil {
		return nil
	}
	cells, err := cv.RawCells()
	if err != nil {
		return nil
	}
	return cells
}
