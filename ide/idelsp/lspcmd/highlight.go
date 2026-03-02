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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

const highlightLocationID = "lsp-highlight"

// HighlightConfig configures the document highlight feature.
type HighlightConfig struct {
	// Delay is the debounce interval after the last cursor
	// movement before a DocumentHighlight RPC is issued.
	// Defaults to 250ms.
	Delay time.Duration

	// ReadAttr is the attribute applied to read occurrences
	// of the highlighted symbol. Also used for text (kind=1)
	// occurrences. Defaults to underline.
	ReadAttr term.Attributes

	// WriteAttr is the attribute applied to write occurrences
	// of the highlighted symbol. Defaults to a dark olive
	// green background.
	WriteAttr term.Attributes
}

// DefaultHighlightConfig returns a HighlightConfig with sensible defaults.
func DefaultHighlightConfig() HighlightConfig {
	return HighlightConfig{
		Delay:     250 * time.Millisecond,
		ReadAttr:  term.Attributes{Attrs: tcell.AttrUnderline},
		WriteAttr: term.Attributes{Bg: tcell.ColorDarkOliveGreen},
	}
}

// SubscribeHighlight subscribes a document-highlight handler
// to cursor and edit events. On each debounced cursor move it
// calls DocumentHighlight and applies the results as a
// location list. Edits immediately clear stale highlights.
func SubscribeHighlight(
	lsp semanticapi.LSP, editor textapi.Editor,
	scheduleNextTick func(func()) bool, cfg HighlightConfig,
) error {
	defaults := DefaultHighlightConfig()
	if cfg.Delay == 0 {
		cfg.Delay = defaults.Delay
	}
	if cfg.ReadAttr == (term.Attributes{}) {
		cfg.ReadAttr = defaults.ReadAttr
	}
	if cfg.WriteAttr == (term.Attributes{}) {
		cfg.WriteAttr = defaults.WriteAttr
	}
	if scheduleNextTick == nil {
		scheduleNextTick = func(fn func()) bool {
			fn()
			return true
		}
	}
	h := &highlightHandler{
		lsp:    lsp,
		editor: editor,
		sched:  scheduleNextTick,
		delay:  cfg.Delay,
		cfg:    cfg,
	}
	return editor.SubscribeEvents(
		[]textapi.EventType{textapi.EventTypeCursor, textapi.EventTypeEdit},
		h,
	)
}

var _ textapi.EventHandler = (*highlightHandler)(nil)

// highlightHandler listens to cursor events, debounces them,
// and issues DocumentHighlight RPCs to highlight occurrences
// of the symbol under the cursor.
type highlightHandler struct {
	mu      sync.Mutex
	timer   *time.Timer
	cancel  context.CancelFunc
	lastURI string
	lastPos semanticapi.Position

	lsp    semanticapi.LSP
	editor textapi.Editor
	sched  func(func()) bool
	delay  time.Duration
	cfg    HighlightConfig
}

// Handle implements textapi.EventHandler.
func (h *highlightHandler) Handle(_ context.Context, ev textapi.Event) bool {
	switch ev.Type {
	case textapi.EventTypeCursor:
		h.onCursor(ev)
	case textapi.EventTypeEdit:
		h.onEdit(ev)
	}
	return false
}

func (h *highlightHandler) onCursor(ev textapi.Event) {
	uri := URIToLSP(ev.URI)
	pos := CoordToPos(ev.From)

	h.mu.Lock()
	defer h.mu.Unlock()

	if uri == h.lastURI && pos == h.lastPos {
		return
	}
	h.lastURI = uri
	h.lastPos = pos

	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}

	if h.timer != nil {
		h.timer.Stop()
	}
	h.timer = time.AfterFunc(h.delay, func() {
		h.fetch(ev.URI, uri, pos)
	})
}

func (h *highlightHandler) onEdit(ev textapi.Event) {
	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	h.mu.Unlock()

	h.sched(func() {
		if err := h.setLocations(ev.URI, nil); err != nil {
			slog.Warn("clear highlights on edit", "err", err)
		}
	})
}

func (h *highlightHandler) fetch(wsURI workspaceapi.URI, uri string, pos semanticapi.Position) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	h.mu.Lock()
	h.cancel = cancel
	h.mu.Unlock()

	defer cancel()

	req := semanticapi.DocumentHighlightParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: uri},
		Position:     pos,
	}
	highlights, err := h.lsp.DocumentHighlight(ctx, req)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("document highlight", "err", err)
		}
		return
	}

	h.mu.Lock()
	stale := uri != h.lastURI || pos != h.lastPos
	h.mu.Unlock()
	if stale {
		return
	}

	h.sched(func() {
		if err := h.applyHighlights(wsURI, highlights); err != nil {
			slog.Warn("apply highlights", "uri", wsURI, "err", err)
		}
	})
}

func (h *highlightHandler) applyHighlights(
	wsURI workspaceapi.URI, highlights []semanticapi.DocumentHighlight,
) error {
	if len(highlights) == 0 {
		return h.setLocations(wsURI, nil)
	}
	locs := make([]textapi.Location, len(highlights))
	for i, hl := range highlights {
		locs[i] = textapi.Location{
			From: PosToCoord(hl.Range.Start),
			To:   PosToCoord(hl.Range.End),
			Attr: h.highlightKindToAttr(hl.Kind),
		}
	}
	return h.setLocations(wsURI, textapi.LocationSlice(locs))
}

func (h *highlightHandler) setLocations(wsURI workspaceapi.URI, list textapi.LocationList) error {
	eh, err := h.editor.Editor(wsURI)
	if err != nil {
		return fmt.Errorf("editor for %s: %w", wsURI, err)
	}
	if err := h.editor.SetLocationList(eh, textapi.LocationPriorityInfo, highlightLocationID, list); err != nil {
		return fmt.Errorf("set location list: %w", err)
	}
	return nil
}

func (h *highlightHandler) highlightKindToAttr(kind semanticapi.DocumentHighlightKind) term.Attributes {
	switch kind {
	case semanticapi.DocumentHighlightKindWrite:
		return h.cfg.WriteAttr
	default:
		return h.cfg.ReadAttr
	}
}
