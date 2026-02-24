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
	// TriggerKey, when non-zero, re-fetches completions on press.
	TriggerKey term.KeyComb

	// Icons maps CompletionItemKind to a display icon string.
	Icons map[semanticapi.CompletionItemKind]string

	// NewSearchList constructs the floating component that displays
	// completion results. Items arrive asynchronously via ch.
	NewSearchList func(
		ch <-chan string, editor textapi.Editor,
		resource textapi.Handler,
	) browserapi.Floating
}

// DefaultCompleteConfig returns a CompleteConfig with sensible defaults.
func DefaultCompleteConfig() CompleteConfig {
	return CompleteConfig{
		NewSearchList: defaultNewSearchList,
		Icons:         defaultIcons(),
	}
}

func defaultIcons() map[semanticapi.CompletionItemKind]string {
	return map[semanticapi.CompletionItemKind]string{
		semanticapi.CompletionItemKindText:          "\ueb8d", // nf-cod-symbol_string
		semanticapi.CompletionItemKindMethod:        "\uea8c", // nf-cod-symbol_method
		semanticapi.CompletionItemKindFunction:      "\uea8c", // nf-cod-symbol_method
		semanticapi.CompletionItemKindConstructor:   "\uea8c", // nf-cod-symbol_method
		semanticapi.CompletionItemKindField:         "\ueb5f", // nf-cod-symbol_field
		semanticapi.CompletionItemKindVariable:      "\uea88", // nf-cod-symbol_variable
		semanticapi.CompletionItemKindClass:         "\ueb5b", // nf-cod-symbol_class
		semanticapi.CompletionItemKindInterface:     "\ueb61", // nf-cod-symbol_interface
		semanticapi.CompletionItemKindModule:        "\uea8b", // nf-cod-symbol_namespace
		semanticapi.CompletionItemKindProperty:      "\ueb65", // nf-cod-symbol_property
		semanticapi.CompletionItemKindUnit:          "\uea96", // nf-cod-symbol_ruler
		semanticapi.CompletionItemKindValue:         "\uea90", // nf-cod-symbol_numeric
		semanticapi.CompletionItemKindEnum:          "\uea95", // nf-cod-symbol_enum
		semanticapi.CompletionItemKindKeyword:       "\ueb62", // nf-cod-symbol_keyword
		semanticapi.CompletionItemKindSnippet:       "\ueb66", // nf-cod-symbol_snippet
		semanticapi.CompletionItemKindColor:         "\ueb5c", // nf-cod-symbol_color
		semanticapi.CompletionItemKindFile:          "\ueb60", // nf-cod-symbol_file
		semanticapi.CompletionItemKindReference:     "\ueb63", // nf-cod-symbol_misc
		semanticapi.CompletionItemKindFolder:        "\ueb60", // nf-cod-symbol_file
		semanticapi.CompletionItemKindEnumMember:    "\ueb5e", // nf-cod-symbol_enum_member
		semanticapi.CompletionItemKindConstant:      "\ueb5d", // nf-cod-symbol_constant
		semanticapi.CompletionItemKindStruct:        "\uea91", // nf-cod-symbol_structure
		semanticapi.CompletionItemKindEvent:         "\uea86", // nf-cod-symbol_event
		semanticapi.CompletionItemKindOperator:      "\ueb64", // nf-cod-symbol_operator
		semanticapi.CompletionItemKindTypeParameter: "\uea92", // nf-cod-symbol_parameter
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

// CompleteHandler creates a textapi.CommandHandler
// for the "complete" subcommand.
func CompleteHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager,
	cfg CompleteConfig, scheduleNextTick func(fn func()) bool,
) textapi.CommandHandler {
	return &completeHandler{
		lsp:              lsp,
		editor:           editor,
		wm:               wm,
		cfg:              cfg,
		scheduleNextTick: scheduleNextTick,
	}
}

var _ textapi.CommandHandler = (*completeHandler)(nil)

type completeHandler struct {
	lsp              semanticapi.LSP
	editor           textapi.Editor
	wm               browserapi.WindowManager
	cfg              CompleteConfig
	scheduleNextTick func(fn func()) bool
}

func (h *completeHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	params := semanticapi.CompletionParams{
		TextDocument: textDocID(cmd.URI),
		Position:     coordToPos(cmd.Cursor.Content),
		Context: &semanticapi.CompletionContext{
			TriggerKind: semanticapi.CompletionTriggerKindInvoked,
		},
	}

	ch := make(chan string, 1)
	floating := h.cfg.NewSearchList(ch, h.editor, cmd.Resource)

	fetchCtx, cancel := context.WithCancel(context.Background())
	wrapped := &closerFloating{
		Floating: floating,
		cancel:   cancel,
	}

	// Inject trigger-key dependencies before wm.Floating so
	// the handler is fully initialised if the window manager
	// renders eagerly. handler.win is set after the call.
	if handler, ok := floating.(*completionHandler); ok {
		handler.lsp = h.lsp
		handler.wm = h.wm
		handler.params = params
		handler.triggerKey = h.cfg.TriggerKey
		handler.icons = h.cfg.Icons
		handler.scheduleNextTick = h.scheduleNextTick
		handler.nextKind = semanticapi.CompletionTriggerKindTriggerForIncompleteCompletions
	}

	win, err := h.wm.Floating(wrapped, browserapi.FloatingConfig{
		Alignment: component.AlignmentLeft | component.AlignmentTop,
		Offset:    term.Coordinates{X: cmd.Cursor.Window.X + 1, Y: cmd.Cursor.Window.Y + 2},
	})
	if err != nil {
		cancel()
		return err
	}

	if handler, ok := floating.(*completionHandler); ok {
		handler.win = win
	}

	wrapped.wg.Add(1)
	go debug.CapturePanicReport(func() {
		defer wrapped.wg.Done()
		defer close(ch)

		result, err := h.lsp.Completion(ctx, params)
		if err != nil {
			slog.Warn("completion fetch", "err", err)
			return
		}
		if len(result.Items) == 0 {
			return
		}

		handler, isDefault := floating.(*completionHandler)
		for _, item := range result.Items {
			label := formatLabel(item, h.cfg.Icons)
			if isDefault {
				handler.addItem(item)
			}
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

// closerFloating wraps a browserapi.Floating to own a goroutine's lifecycle.
type closerFloating struct {
	browserapi.Floating
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (c *closerFloating) Close() error {
	c.cancel()
	c.wg.Wait()
	return c.Floating.Close()
}

// completionHandler is a floating window that
// displays and allows selecting completion items.
type completionHandler struct {
	mu       sync.Mutex
	items    []semanticapi.CompletionItem
	list     *component.FocusList
	ch       <-chan string
	editor   textapi.Editor
	resource textapi.Handler
	width    int
	height   int

	// TriggerKey dependencies (injected by HandleCommand).
	win              browserapi.Window
	lsp              semanticapi.LSP
	wm               browserapi.WindowManager
	params           semanticapi.CompletionParams
	triggerKey       term.KeyComb
	icons            map[semanticapi.CompletionItemKind]string
	nextKind         semanticapi.CompletionTriggerKind
	scheduleNextTick func(fn func()) bool

	// Goroutine lifecycle for trigger re-fetches.
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func defaultNewSearchList(
	ch <-chan string, editor textapi.Editor,
	resource textapi.Handler,
) browserapi.Floating {
	list := component.NewFocusList()
	list.SetFocusAttr(term.Attributes{Fg: tcell.ColorWhite})
	return &completionHandler{
		list:     list,
		ch:       ch,
		editor:   editor,
		resource: resource,
		width:    20,
		height:   1,
	}
}

// newCompletionHandler creates a pre-populated completionHandler.
// Used in tests that need a synchronously filled handler.
func newCompletionHandler(
	items []semanticapi.CompletionItem,
	icons map[semanticapi.CompletionItemKind]string,
	editor textapi.Editor, resource textapi.Handler,
) *completionHandler {
	list := component.NewFocusList()
	list.SetFocusAttr(term.Attributes{Fg: tcell.ColorWhite})
	cfg := component.StringConfig{
		Attributes: term.Attributes{Fg: tcell.ColorGray},
	}
	maxW := 0
	for _, item := range items {
		label := formatLabel(item, icons)
		list.PushBack(component.NewStringWithConfig(label, cfg))
		if n := utf8.RuneCountInString(label); n > maxW {
			maxW = n
		}
	}
	h := min(len(items), 15)
	w := maxW + 2
	list.Resize(w, h)
	return &completionHandler{
		list:     list,
		items:    items,
		editor:   editor,
		resource: resource,
		width:    w,
		height:   h,
	}
}

func (c *completionHandler) addItem(item semanticapi.CompletionItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, item)
}

func (c *completionHandler) drainChannel() {
	if c.ch == nil {
		return
	}
	cfg := component.StringConfig{
		Attributes: term.Attributes{Fg: tcell.ColorGray},
	}
	for {
		select {
		case label, ok := <-c.ch:
			if !ok {
				c.ch = nil
				return
			}
			c.list.PushBack(
				component.NewStringWithConfig(label, cfg),
			)
			n := utf8.RuneCountInString(label) + 2
			if n > c.width {
				c.width = n
			}
			count := c.list.Len()
			c.height = min(count, 15)
			c.list.Resize(c.width, c.height)
		default:
			return
		}
	}
}

func (c *completionHandler) Handle(
	ev term.Event,
) (exit, handled bool) {
	c.drainChannel()
	if ev.Type != term.EventKey {
		return false, false
	}
	kc := ev.KeyComb()
	zeroKey := term.KeyComb{}
	if c.triggerKey != zeroKey && kc == c.triggerKey {
		c.handleTrigger()
		return false, true
	}
	switch ev.Key {
	case term.KeyEsc:
		return true, true
	case term.KeyEnter:
		if err := c.applyItem(); err != nil {
			slog.Warn("completion apply", "err", err)
		}
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

func (c *completionHandler) handleTrigger() {
	// Toggle between IncompleteCompletions and Invoked.
	if c.nextKind == semanticapi.CompletionTriggerKindTriggerForIncompleteCompletions {
		c.nextKind = semanticapi.CompletionTriggerKindInvoked
	} else {
		c.nextKind = semanticapi.CompletionTriggerKindTriggerForIncompleteCompletions
	}

	// Cancel any previous trigger goroutine. Do not Wait here
	// to avoid blocking the event loop; Close() waits instead.
	if c.cancel != nil {
		c.cancel()
	}

	triggerCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	params := c.params
	params.Context = &semanticapi.CompletionContext{
		TriggerKind: c.nextKind,
	}

	// Capture nextKind before spawning the goroutine to avoid
	// a data race if the user presses the trigger key again.
	nextKind := c.nextKind

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		result, err := c.lsp.Completion(triggerCtx, params)
		if err != nil {
			slog.Warn("completion re-fetch", "err", err)
			return
		}

		c.scheduleNextTick(func() {
			ch := make(chan string, 1)
			close(ch) // Items are populated directly; no async delivery needed.
			newHandler := defaultNewSearchList(ch, c.editor, c.resource)
			inner := newHandler.(*completionHandler)
			inner.win = c.win
			inner.lsp = c.lsp
			inner.wm = c.wm
			inner.params = c.params
			inner.triggerKey = c.triggerKey
			inner.icons = c.icons
			inner.scheduleNextTick = c.scheduleNextTick
			inner.nextKind = nextKind

			for _, item := range result.Items {
				label := formatLabel(item, c.icons)
				inner.addItem(item)
				inner.list.PushBack(
					component.NewStringWithConfig(label, component.StringConfig{
						Attributes: term.Attributes{Fg: tcell.ColorGray},
					}),
				)
				n := utf8.RuneCountInString(label) + 2
				if n > inner.width {
					inner.width = n
				}
			}
			count := inner.list.Len()
			inner.height = min(count, 15)
			if inner.height == 0 {
				inner.height = 1
			}
			inner.list.Resize(inner.width, inner.height)

			if err := c.wm.SetWindowContent(c.win, newHandler); err != nil {
				slog.Warn("completion set window content", "err", err)
			}
		})
	}()
}

func (c *completionHandler) Cursor() (
	term.Coordinates, term.CursorStyle, bool,
) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

func (c *completionHandler) Selection() (string, bool) {
	return "", false
}

func (c *completionHandler) Resize(width, height int) {
	c.width = width
	c.height = height
	c.list.Resize(width, height)
}

func (c *completionHandler) Draw(w term.Writer) {
	c.list.Draw(w)
}

func (c *completionHandler) Dimensions() (int, int) {
	return c.width, c.height
}

func (c *completionHandler) Close() error {
	if c.cancel != nil {
		c.cancel()
		c.wg.Wait()
	}
	return nil
}

func (c *completionHandler) applyItem() error {
	c.mu.Lock()
	items := c.items
	idx := c.list.FocusOffset()
	c.mu.Unlock()

	if idx >= len(items) {
		return nil
	}
	item := items[idx]
	ce := c.editor.CellEditor(c.resource)
	ctx := context.Background()
	switch {
	case item.TextEdit != nil:
		edits := []semanticapi.TextEdit{*item.TextEdit}
		return applyEdits(ctx, ce, edits)
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
