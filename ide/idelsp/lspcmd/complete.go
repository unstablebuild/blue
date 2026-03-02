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
	"log/slog"
	"sync"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/debug"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// CompleteConfig configures the "complete" subcommand.
type CompleteConfig struct {
	// Icons maps CompletionItemKind to a display icon string.
	Icons map[semanticapi.CompletionItemKind]string
}

// DefaultCompleteConfig returns a CompleteConfig with sensible defaults.
func DefaultCompleteConfig() CompleteConfig {
	return CompleteConfig{
		Icons: defaultIcons(),
	}
}

func defaultIcons() map[semanticapi.CompletionItemKind]string {
	return map[semanticapi.CompletionItemKind]string{
		semanticapi.CompletionItemKindText:        "󰉿",  // nf-cod-symbol_text
		semanticapi.CompletionItemKindMethod:      ".󰊕", // nf-cod-symbol_method
		semanticapi.CompletionItemKindFunction:    "󰊕",  // nf-cod-symbol_function
		semanticapi.CompletionItemKindConstructor: "󰒓",  // nf-cod-symbol_constructor

		semanticapi.CompletionItemKindField:    "󰜢", // nf-cod-symbol_field
		semanticapi.CompletionItemKindVariable: "󰀫", // nf-cod-symbol_variable
		semanticapi.CompletionItemKindConstant: "󰏿", // nf-cod-symbol_constant
		semanticapi.CompletionItemKindProperty: "󰆧", // nf-cod-symbol_property

		semanticapi.CompletionItemKindClass:         "󰠱", // nf-cod-symbol_class
		semanticapi.CompletionItemKindStruct:        "󰙅", // nf-cod-symbol_structure
		semanticapi.CompletionItemKindInterface:     "󰜰", // nf-cod-symbol_interface
		semanticapi.CompletionItemKindTypeParameter: "󰆩", // nf-cod-symbol_parameter

		semanticapi.CompletionItemKindModule: "󰅩", // nf-cod-symbol_namespace
		semanticapi.CompletionItemKindUnit:   "󰑭", // nf-cod-symbol_ruler
		semanticapi.CompletionItemKindValue:  "󰎠", // nf-cod-symbol_numeric

		semanticapi.CompletionItemKindEnum:       "󰕘", // nf-cod-symbol_enum
		semanticapi.CompletionItemKindEnumMember: "󰕘", // nf-cod-symbol_enum_member (shares icon intentionally)

		semanticapi.CompletionItemKindKeyword:  "󰌋", // nf-cod-symbol_keyword
		semanticapi.CompletionItemKindOperator: "󰆕", // nf-cod-symbol_operator

		semanticapi.CompletionItemKindSnippet: "󰘍", // nf-cod-symbol_snippet
		semanticapi.CompletionItemKindColor:   "󰏘", // nf-cod-symbol_color

		semanticapi.CompletionItemKindFile:      "󰈙", // nf-cod-symbol_file
		semanticapi.CompletionItemKindFolder:    "󰉋", // nf-cod-folder
		semanticapi.CompletionItemKindReference: "󰈇", // nf-cod-symbol_reference
		semanticapi.CompletionItemKindEvent:     "󰉁", // nf-cod-symbol_event
	}
}

func formatLabel(
	item semanticapi.CompletionItem,
	icons map[semanticapi.CompletionItemKind]string,
) string {
	if icon, ok := icons[item.Kind]; ok {
		return icon + " " + item.Label
	}
	return item.Label
}

// CompleteHandler creates a textapi.CommandHandler for the "complete" subcommand.
func CompleteHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, cfg CompleteConfig,
	interrupter term.Interrupter,
) textapi.CommandHandler {
	return &completeHandler{
		lsp:         lsp,
		editor:      editor,
		wm:          wm,
		cfg:         cfg,
		interrupter: interrupter,
	}
}

var _ textapi.CommandHandler = (*completeHandler)(nil)

type completeHandler struct {
	lsp         semanticapi.LSP
	editor      textapi.Editor
	wm          browserapi.WindowManager
	cfg         CompleteConfig
	interrupter term.Interrupter
}

func (h *completeHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	params := semanticapi.CompletionParams{
		TextDocument: TextDocID(cmd.URI),
		Position:     CoordToPos(cmd.Cursor.Content),
		Context: &semanticapi.CompletionContext{
			TriggerKind: semanticapi.CompletionTriggerKindInvoked,
		},
	}

	ch := make(chan string, 1)
	fetchCtx, cancel := context.WithCancel(context.Background())
	fetchDone, fetchDoneCancel := context.WithCancel(context.Background())
	drainDone, drainDoneCancel := context.WithCancel(context.Background())

	list := component.NewFocusList()
	list.SetFocusAttr(term.Attributes{Fg: tcell.ColorWhite})

	floating := &completionHandler{
		list:        list,
		drainDone:   drainDone,
		interrupter: h.interrupter,
		cancel:      cancel,
		fetchDone:   fetchDone,
		icons:       h.cfg.Icons,
		editor:      h.editor,
		resource:    cmd.Resource,
	}

	go debug.CapturePanicReport(func() {
		defer drainDoneCancel()
		floating.drainLoop(ch)
	})

	_, err := h.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentLeft | component.AlignmentTop,
		Offset:    term.Coordinates{X: cmd.Cursor.Window.X + 1, Y: cmd.Cursor.Window.Y + 2},
	})
	if err != nil {
		cancel()
		fetchDoneCancel()
		return err
	}

	go debug.CapturePanicReport(func() {
		defer fetchDoneCancel()
		defer close(ch)

		result, err := h.lsp.Completion(ctx, params)
		if err != nil {
			slog.Warn("completion fetch", "err", err)
			return
		}
		for _, item := range result.Items {
			label := formatLabel(item, h.cfg.Icons)
			floating.itemsMu.Lock()
			floating.items = append(floating.items, item)
			floating.itemsMu.Unlock()
			select {
			case ch <- label:
			case <-fetchCtx.Done():
				return
			}
		}
	})

	return nil
}

func (h *completeHandler) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// completionHandler is a floating window that displays completion
// items, allows selecting one, and applies the chosen edit.
type completionHandler struct {
	// UI state (protected by mu).
	mu          sync.Mutex
	labels      []string
	list        *component.FocusList
	selected    bool
	interrupter term.Interrupter

	// Goroutine lifecycle.
	cancel    context.CancelFunc
	fetchDone context.Context
	drainDone context.Context

	// Completion items (protected by itemsMu).
	itemsMu  sync.Mutex
	items    []semanticapi.CompletionItem
	icons    map[semanticapi.CompletionItemKind]string
	editor   textapi.Editor
	resource textapi.Handler
}

// newCompletionHandler creates a pre-populated completionHandler.
// Used in tests that need a synchronously filled handler.
func newCompletionHandler(labels []string) *completionHandler {
	list := component.NewFocusList()
	list.SetFocusAttr(term.Attributes{Fg: tcell.ColorWhite})
	strCfg := component.StringConfig{
		Attributes: term.Attributes{Fg: tcell.ColorGray},
	}
	for _, label := range labels {
		list.PushBack(component.NewStringWithConfig(label, strCfg))
	}
	h := &completionHandler{
		list:        list,
		labels:      labels,
		interrupter: term.NopInterrupter(),
	}
	return h
}

func (c *completionHandler) drainLoop(ch <-chan string) {
	strCfg := component.StringConfig{
		Attributes: term.Attributes{Fg: tcell.ColorGray},
	}
	for label := range ch {
		c.mu.Lock()
		c.labels = append(c.labels, label)
		c.list.PushBack(component.NewStringWithConfig(label, strCfg))
		for draining := true; draining; {
			select {
			case l, ok := <-ch:
				if !ok {
					draining = false
				} else {
					c.labels = append(c.labels, l)
					c.list.PushBack(component.NewStringWithConfig(l, strCfg))
				}
			default:
				draining = false
			}
		}
		c.mu.Unlock()
		_ = c.interrupter.Interrupt(context.Background())
	}
}

// Focus returns the label of the focused item. Returns ("", false)
// when the user dismissed without selecting (Esc).
func (c *completionHandler) Focus() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.selected {
		return "", false
	}
	idx := c.list.FocusOffset()
	if idx >= len(c.labels) {
		return "", false
	}
	return c.labels[idx], true
}

func (c *completionHandler) Handle(ev term.Event) (bool, bool) {
	exit, handled := c.handleKey(ev)
	if !exit {
		return exit, handled
	}
	label, ok := c.Focus()
	if !ok {
		return exit, handled
	}
	if err := c.applyItem(label); err != nil {
		slog.Warn("completion apply", "err", err)
	}
	return exit, handled
}

func (c *completionHandler) handleKey(ev term.Event) (exit, handled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ev.Type != term.EventKey {
		return false, false
	}
	switch ev.Key {
	case term.KeyEsc:
		return true, true
	case term.KeyEnter, term.KeyTab:
		c.selected = true
		return true, true
	case term.KeyArrowDown:
		c.list.FocusDown()
		return false, true
	case term.KeyArrowUp:
		c.list.FocusUp()
		return false, true
	}
	if ev.Mod == term.ModCtrl {
		switch ev.Ch {
		case 'j':
			c.list.FocusDown()
			return false, true
		case 'k':
			c.list.FocusUp()
			return false, true
		}
	}
	return false, false
}

func (c *completionHandler) applyItem(label string) error {
	c.itemsMu.Lock()
	var item semanticapi.CompletionItem
	var found bool
	for _, it := range c.items {
		if formatLabel(it, c.icons) == label {
			item = it
			found = true
			break
		}
	}
	c.itemsMu.Unlock()
	if !found {
		return nil
	}

	ce := c.editor.CellEditor(c.resource)
	ctx := context.Background()
	switch {
	case item.TextEdit != nil:
		edits := []semanticapi.TextEdit{*item.TextEdit}
		return ApplyEdits(ctx, ce, edits)
	case item.InsertText != "":
		cur, err := c.editor.Cursor(c.resource)
		if err != nil {
			return err
		}
		_, _, _, err = ce.Edit(ctx, cur, cur, item.InsertText)
		return err
	default:
		cur, err := c.editor.Cursor(c.resource)
		if err != nil {
			return err
		}
		_, _, _, err = ce.Edit(ctx, cur, cur, item.Label)
		return err
	}
}

func (c *completionHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

func (c *completionHandler) Selection() (string, bool) {
	return "", false
}

func (c *completionHandler) Resize(width, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list.Resize(width, height)
}

func (c *completionHandler) Draw(w term.Writer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list.Draw(w)
}

func (c *completionHandler) Dimensions() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.contentDimensions()
}

func (c *completionHandler) contentDimensions() (int, int) {
	maxW := 0
	for _, label := range c.labels {
		if n := utf8.RuneCountInString(label); n > maxW {
			maxW = n
		}
	}
	return maxW + 2, min(len(c.labels), 15)
}

func (c *completionHandler) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.fetchDone != nil {
		<-c.fetchDone.Done()
	}
	if c.drainDone != nil {
		<-c.drainDone.Done()
	}
	return nil
}
