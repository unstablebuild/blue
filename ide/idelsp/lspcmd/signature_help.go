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
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/debug"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// SignatureHelpConfig configures the "signature-help" subcommand.
type SignatureHelpConfig struct {
	// ActiveParamAttr is applied to the active parameter within
	// the signature label.
	ActiveParamAttr term.Attributes

	// AutoTrigger enables automatic signature help when a
	// trigger character is typed. Defaults to true.
	AutoTrigger bool

	// TriggerCharacters lists the characters that trigger
	// automatic signature help (e.g. "(", ","). Populated
	// by the caller from server capabilities.
	TriggerCharacters []string
}

// DefaultSignatureHelpConfig returns a SignatureHelpConfig with
// bold+underline defaults for the active parameter highlight.
func DefaultSignatureHelpConfig() SignatureHelpConfig {
	return SignatureHelpConfig{
		ActiveParamAttr: term.Attributes{
			Attrs: tcell.AttrBold | tcell.AttrUnderline,
		},
		AutoTrigger: true,
	}
}

const signatureHelpLocationID = "lsp-signature-help"

// SignatureHelpHandler creates a textapi.CommandHandler for the
// "signature-help" subcommand. When AutoTrigger is enabled and
// TriggerCharacters are provided, it subscribes to edit and
// cursor events and automatically shows signature help via a
// location list when a trigger character is typed.
func SignatureHelpHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager,
	scheduleNextTick func(func()) bool,
	cfg SignatureHelpConfig,
) textapi.CommandHandler {
	h := &signatureHelpHandler{
		lsp:    lsp,
		editor: editor,
		wm:     wm,
		cfg:    cfg,
		sched:  scheduleNextTick,
	}
	if !cfg.AutoTrigger || len(cfg.TriggerCharacters) == 0 {
		return h
	}
	triggerSet := make(map[string]struct{}, len(cfg.TriggerCharacters))
	for _, ch := range cfg.TriggerCharacters {
		triggerSet[ch] = struct{}{}
	}
	h.triggerSet = triggerSet
	err := editor.SubscribeEvents(
		[]textapi.EventType{textapi.EventTypeEdit, textapi.EventTypeCursor}, h,
	)
	if err != nil {
		slog.Warn("signature help subscribe events", "err", err)
	}
	return h
}

var (
	_ textapi.CommandHandler = (*signatureHelpHandler)(nil)
	_ textapi.EventHandler   = (*signatureHelpHandler)(nil)
	_ browserapi.Floating    = (*signatureHelpFloating)(nil)
)

type signatureHelpHandler struct {
	mu       sync.Mutex
	cancel   context.CancelFunc
	editSeen bool // suppresses cursor-clear after edits
	active   bool // true when a signature help location is showing

	lsp        semanticapi.LSP
	editor     textapi.Editor
	wm         browserapi.WindowManager
	cfg        SignatureHelpConfig
	triggerSet map[string]struct{}
	sched      func(func()) bool
}

// Handle implements textapi.EventHandler.
func (h *signatureHelpHandler) Handle(_ context.Context, ev textapi.Event) bool {
	switch ev.Type {
	case textapi.EventTypeEdit:
		h.onEdit(ev)
	case textapi.EventTypeCursor:
		h.onCursor(ev)
	}
	return false
}

// onEdit fires on every edit event. Trigger characters always
// launch a fetch. Non-trigger characters re-fetch only when
// signature help is already showing, so the LSP server can
// decide whether the cursor is still inside a function call.
func (h *signatureHelpHandler) onEdit(ev textapi.Event) {
	if ev.Content == "" {
		return
	}
	slog.Debug("signature help: onEdit", "content", len(ev.Content))
	lastRune, _ := utf8.DecodeLastRuneInString(ev.Content)
	if lastRune == utf8.RuneError {
		slog.Debug("signature help: last rune error")
		return
	}

	_, isTrigger := h.triggerSet[string(lastRune)]

	h.mu.Lock()
	h.editSeen = true
	if !isTrigger && !h.active {
		h.mu.Unlock()
		slog.Debug("signature help: not a trigger and not active", "rune", string(lastRune))
		return
	}
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	h.mu.Unlock()

	go debug.CapturePanicReport(func() { h.fetch(ev) })
}

// onCursor cancels any in-flight fetch and clears the location
// list, unless the cursor moved as a result of an edit (in which
// case the re-fetch from onEdit handles it).
func (h *signatureHelpHandler) onCursor(ev textapi.Event) {
	h.mu.Lock()
	if h.editSeen {
		h.editSeen = false
		h.mu.Unlock()
		return
	}
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	h.active = false
	h.mu.Unlock()

	h.sched(func() {
		h.clearLocation(ev.URI)
	})
}

func (h *signatureHelpHandler) fetch(ev textapi.Event) {
	slog.Debug("signature help: fetching")
	ctx, cancel := context.WithCancel(context.Background())

	h.mu.Lock()
	h.cancel = cancel
	h.mu.Unlock()

	defer cancel()

	params := semanticapi.SignatureHelpParams{
		TextDocument: TextDocID(ev.URI),
		Position:     CoordToPos(ev.To),
	}
	result, err := h.lsp.SignatureHelp(ctx, params)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("signature help auto-trigger", "err", err)
		} else {
			slog.Debug("signature help auto-trigger", "err", err)
		}
		return
	}
	if result == nil || len(result.Signatures) == 0 {
		slog.Debug("signature help: no signatures")
		h.mu.Lock()
		h.active = false
		h.mu.Unlock()
		h.sched(func() { h.clearLocation(ev.URI) })
		return
	}

	msg := FormatSignatureMessage(result)
	from := ev.To
	to := from
	to.X++
	loc := textapi.Location{
		From:    from,
		To:      to,
		Message: msg,
	}
	slog.Debug("setting signature help", "location", loc)

	h.mu.Lock()
	h.active = true
	h.mu.Unlock()

	h.sched(func() {
		h.setLocation(ev.URI, textapi.LocationSlice([]textapi.Location{loc}))
	})
}

func (h *signatureHelpHandler) setLocation(wsURI workspaceapi.URI, list textapi.LocationList) {
	eh, err := h.editor.Editor(wsURI)
	if err != nil {
		slog.Warn("signature help editor", "err", err)
		return
	}
	if err := h.editor.SetLocationList(eh, textapi.LocationPriorityInfo,
		signatureHelpLocationID, list); err != nil {
		slog.Warn("signature help set location list", "err", err)
	}
}

func (h *signatureHelpHandler) clearLocation(wsURI workspaceapi.URI) {
	h.setLocation(wsURI, nil)
}

// FormatSignatureMessage formats a SignatureHelp result as a
// markdown string with **bold** around the active parameter.
func FormatSignatureMessage(result *semanticapi.SignatureHelp) string {
	idx := int(result.ActiveSignature)
	if idx >= len(result.Signatures) {
		idx = len(result.Signatures) - 1
	}
	if idx < 0 {
		idx = 0
	}
	sig := result.Signatures[idx]
	paramIdx := int(result.ActiveParameter)
	paramStart, paramEnd := findParamRuneRange(sig, paramIdx)

	var b strings.Builder
	if paramStart >= 0 {
		i := 0
		for _, r := range sig.Label {
			if i == paramStart {
				b.WriteString("**")
			}
			b.WriteRune(r)
			i++
			if i == paramEnd {
				b.WriteString("**")
			}
		}
	} else {
		b.WriteString(sig.Label)
	}

	if len(result.Signatures) > 1 {
		fmt.Fprintf(&b, " %d/%d", idx+1, len(result.Signatures))
	}

	paramDoc := activeParamDoc(sig, paramIdx)
	if paramDoc != "" {
		b.WriteByte('\n')
		b.WriteString(paramDoc)
	}

	return b.String()
}

func (h *signatureHelpHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if cmd.Resource == nil {
		return nil
	}
	params := semanticapi.SignatureHelpParams{
		TextDocument: TextDocID(cmd.URI),
		Position:     CoordToPos(cmd.Cursor.Content),
	}
	result, err := h.lsp.SignatureHelp(ctx, params)
	if err != nil {
		return err
	}
	if result == nil || len(result.Signatures) == 0 {
		return nil
	}
	f := newSignatureHelpFloating(result, h.cfg, h.wm)
	win, err := h.wm.Floating(f, browserapi.FloatingConfig{
		Alignment: component.AlignmentLeft | component.AlignmentTop,
		Offset:    term.Coordinates{X: cmd.Cursor.Window.X + 1, Y: cmd.Cursor.Window.Y + 2},
	})
	if err != nil {
		return err
	}
	f.win = win
	return nil
}

func (h *signatureHelpHandler) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

type signatureHelpFloating struct {
	result    *semanticapi.SignatureHelp
	activeIdx int
	cfg       SignatureHelpConfig
	wm        browserapi.WindowManager
	win       browserapi.Window
	width     int
	height    int
}

func newSignatureHelpFloating(
	result *semanticapi.SignatureHelp, cfg SignatureHelpConfig,
	wm browserapi.WindowManager,
) *signatureHelpFloating {
	idx := int(result.ActiveSignature)
	if idx >= len(result.Signatures) {
		idx = len(result.Signatures) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return &signatureHelpFloating{
		result:    result,
		activeIdx: idx,
		cfg:       cfg,
		wm:        wm,
	}
}

func (f *signatureHelpFloating) Handle(ev term.Event) (bool, bool) {
	if ev.Type != term.EventKey {
		return false, false
	}
	switch ev.Key {
	case term.KeyArrowUp:
		if f.activeIdx > 0 {
			f.activeIdx--
		}
		return false, true
	case term.KeyArrowDown:
		if f.activeIdx < len(f.result.Signatures)-1 {
			f.activeIdx++
		}
		return false, true
	default:
		return true, true
	}
}

func (f *signatureHelpFloating) Draw(w term.Writer) {
	sig := f.result.Signatures[f.activeIdx]
	paramIdx := int(f.result.ActiveParameter)
	paramStart, paramEnd := findParamRuneRange(sig, paramIdx)

	label := sig.Label
	counter := ""
	if len(f.result.Signatures) > 1 {
		counter = fmt.Sprintf(" %d/%d", f.activeIdx+1, len(f.result.Signatures))
	}

	x := 0
	for _, r := range label {
		cell := term.Cell{Ch: r, Width: 1}
		if paramStart >= 0 && x >= paramStart && x < paramEnd {
			cell.Attributes = term.AttributesUnion(cell.Attributes, f.cfg.ActiveParamAttr)
		}
		w.SetCell(term.Coordinates{X: x, Y: 0}, cell)
		x++
	}

	if counter != "" {
		grayAttr := term.Attributes{Fg: tcell.ColorGray}
		for _, r := range counter {
			w.SetCell(term.Coordinates{X: x, Y: 0}, term.Cell{
				Ch: r, Width: 1, Attributes: grayAttr,
			})
			x++
		}
	}

	paramDoc := activeParamDoc(sig, paramIdx)
	if paramDoc != "" {
		grayAttr := term.Attributes{Fg: tcell.ColorGray}
		dx := 0
		for _, r := range paramDoc {
			w.SetCell(term.Coordinates{X: dx, Y: 1}, term.Cell{
				Ch: r, Width: 1, Attributes: grayAttr,
			})
			dx++
		}
	}
}

func (f *signatureHelpFloating) Dimensions() (int, int) {
	sig := f.result.Signatures[f.activeIdx]
	labelW := utf8.RuneCountInString(sig.Label)
	if len(f.result.Signatures) > 1 {
		counter := fmt.Sprintf(" %d/%d", f.activeIdx+1, len(f.result.Signatures))
		labelW += utf8.RuneCountInString(counter)
	}

	paramDoc := activeParamDoc(sig, int(f.result.ActiveParameter))
	docW := utf8.RuneCountInString(paramDoc)

	w := max(labelW, docW)
	h := 1
	if paramDoc != "" {
		h = 2
	}
	return w, h
}

func (f *signatureHelpFloating) Resize(w, h int) {
	f.width = w
	f.height = h
}

func (f *signatureHelpFloating) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

func (f *signatureHelpFloating) Selection() (string, bool) {
	return "", false
}

func (f *signatureHelpFloating) Close() error {
	if f.win != nil {
		return f.wm.CloseWindow(f.win)
	}
	return nil
}

// findParamRuneRange returns the start and end rune offsets of
// the active parameter's label within the signature label.
// Returns (-1, -1) if the parameter cannot be found.
func findParamRuneRange(
	sig semanticapi.SignatureInformation, paramIdx int,
) (int, int) {
	if paramIdx < 0 || paramIdx >= len(sig.Parameters) {
		return -1, -1
	}
	param := sig.Parameters[paramIdx]
	if param.LabelOffsets != nil {
		start := utf8.RuneCountInString(sig.Label[:param.LabelOffsets[0]])
		end := utf8.RuneCountInString(sig.Label[:param.LabelOffsets[1]])
		return start, end
	}
	if param.Label == "" {
		return -1, -1
	}
	byteIdx := strings.Index(sig.Label, param.Label)
	if byteIdx < 0 {
		return -1, -1
	}
	start := utf8.RuneCountInString(sig.Label[:byteIdx])
	end := start + utf8.RuneCountInString(param.Label)
	return start, end
}

// activeParamDoc returns the documentation string for the active
// parameter, or "" if none is available.
func activeParamDoc(
	sig semanticapi.SignatureInformation, paramIdx int,
) string {
	if paramIdx < 0 || paramIdx >= len(sig.Parameters) {
		return ""
	}
	p := sig.Parameters[paramIdx]
	if p.Documentation != nil && p.Documentation.Value != "" {
		return p.Documentation.Value
	}
	return p.DocumentationString
}
