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
	"bufio"
	"os"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const (
	previewContextLines = 21 // +/- 10 lines around the reference
	minPreviewWidth     = 80
	maxListHeight       = 15
)

// LocationsConfig configures the appearance of a locations floating handler.
type LocationsConfig struct {
	ListTextAttr  term.Attributes
	ListFocusAttr term.Attributes
	PreviewAttr   term.Attributes
}

type locationsFloatingHandler struct {
	entries []locationEntry
	list    *component.FocusList
	editor  textapi.Editor
	opener  browserapi.ResourceOpener
	wm      browserapi.WindowManager
	notify  browserapi.Notifications
	fs      workspaceapi.FileSystem

	preview      []string  // lines of the currently previewed file
	previewRunes [][]rune  // parallel rune slices for allocation-free Draw
	prevURI      string    // URI of the currently loaded preview

	idealW   int // pre-computed ideal width from entries
	actualW  int // allocated width from Resize
	actualH  int // allocated height from Resize
	previewH int // computed preview height from Resize
	listH    int // computed list height from Resize

	previewAttr term.Attributes
}

func newLocationsFloatingHandler(
	entries []locationEntry,
	opener browserapi.ResourceOpener, wm browserapi.WindowManager,
	editor textapi.Editor, notify browserapi.Notifications,
	fs workspaceapi.FileSystem, cfg LocationsConfig,
) *locationsFloatingHandler {
	list := &component.FocusList{}
	list.InitWithAttr(cfg.ListTextAttr, cfg.ListFocusAttr)
	idealW := 0
	for _, e := range entries {
		list.PushBack(component.NewResponsiveString(e.display, component.StringResponsiveConfig{}))
		if w := utf8.RuneCountInString(e.display); w > idealW {
			idealW = w
		}
	}
	if idealW < minPreviewWidth {
		idealW = minPreviewWidth
	}
	handler := &locationsFloatingHandler{
		entries:     entries,
		list:        list,
		editor:      editor,
		opener:      opener,
		wm:          wm,
		notify:      notify,
		fs:          fs,
		idealW:      idealW,
		previewAttr: cfg.PreviewAttr,
	}
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
			if err := navigateTo(l.entries[idx], l.opener, l.wm, l.editor); err != nil {
				l.notify.Notify(browserapi.LevelError, "navigate: %s", err) //nolint:errcheck
			}
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
	return false, false
}

func (l *locationsFloatingHandler) Draw(w term.Writer) {
	l.drawPreview(w)
	vw := &component.VirtualWriter{
		Writer: w,
		Offset: term.Coordinates{Y: l.previewH},
		Width:  l.actualW,
		Height: l.listH,
	}
	l.list.Draw(vw)
}

func (l *locationsFloatingHandler) drawPreview(w term.Writer) {
	idx := l.list.FocusOffset()
	if idx >= len(l.entries) || l.preview == nil {
		return
	}
	entry := l.entries[idx]
	targetLine := int(entry.rng.Start.Line)
	if targetLine >= len(l.preview) {
		targetLine = len(l.preview) - 1
		if targetLine < 0 {
			targetLine = 0
		}
	}
	startLine := targetLine - l.previewH/2
	if startLine < 0 {
		startLine = 0
	}
	if startLine+l.previewH > len(l.preview) {
		startLine = len(l.preview) - l.previewH
		if startLine < 0 {
			startLine = 0
		}
	}
	for row := range l.previewH {
		srcLine := startLine + row
		if srcLine >= len(l.preview) {
			break
		}
		isTarget := srcLine == targetLine
		runes := l.previewRunes[srcLine]
		for x := range l.actualW {
			ch := ' '
			if x < len(runes) {
				ch = runes[x]
			}
			cell := term.Cell{Ch: ch, Width: 1}
			if isTarget {
				cell.Attributes = l.previewAttr
			}
			w.SetCell(term.Coordinates{X: x, Y: row}, cell)
		}
	}
}

func (l *locationsFloatingHandler) Dimensions() (int, int) {
	listH := len(l.entries)
	if listH > maxListHeight {
		listH = maxListHeight
	}
	return l.idealW, listH + previewContextLines
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
	l.actualW = w
	l.actualH = h
	l.previewH = previewContextLines
	if l.previewH > h-1 {
		l.previewH = h - 1
	}
	if l.previewH < 0 {
		l.previewH = 0
	}
	l.listH = h - l.previewH
	if l.listH < 1 {
		l.listH = 1
	}
	l.list.Resize(w, l.listH)
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
		l.preview = nil
		l.previewRunes = nil
		l.prevURI = ""
		return
	}
	f, err := l.fs.OpenFile(uri.Path(), os.O_RDONLY, 0)
	if err != nil {
		l.preview = nil
		l.previewRunes = nil
		l.prevURI = ""
		return
	}
	defer f.Close() //nolint:errcheck
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		l.preview = nil
		l.previewRunes = nil
		l.prevURI = ""
		return
	}
	runes := make([][]rune, len(lines))
	for i, line := range lines {
		runes[i] = []rune(line)
	}
	l.preview = lines
	l.previewRunes = runes
	l.prevURI = entry.uri
}
