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

	mdcomp "github.com/unstablebuild/blue/tui/component/markdown"
	mdhandler "github.com/unstablebuild/blue/tui/handler/markdown"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// HoverConfig configures the "hover" subcommand.
type HoverConfig struct {
	// MarkdownConfig configures the markdown component used to render
	// hover results with MarkupKindMarkdown content.
	MarkdownConfig mdcomp.Config
	// MarkdownHandlerOptions configures the markdown handler used to
	// render hover results with MarkupKindMarkdown content.
	MarkdownHandlerOptions []mdhandler.Option
}

// DefaultHoverConfig returns a HoverConfig with sensible defaults.
func DefaultHoverConfig() HoverConfig {
	return HoverConfig{
		MarkdownConfig: mdcomp.DefaultConfig(),
	}
}

// HoverHandler creates a textapi.CommandHandler for the "hover" subcommand.
// It calls the LSP hover request and displays the result in a floating window.
func HoverHandler(
	lsp semanticapi.LSP, wm browserapi.WindowManager,
	notify browserapi.Notifications, fs workspaceapi.FileSystem,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser, cfg HoverConfig,
) textapi.CommandHandler {
	return &hoverHandler{
		lsp: lsp, wm: wm, notify: notify, fs: fs,
		scheduleNextTick: scheduleNextTick, parser: parser, cfg: cfg,
	}
}

var (
	_ textapi.CommandHandler = (*hoverHandler)(nil)
	_ browserapi.Floating    = (*hoverFloating)(nil)
)

type hoverHandler struct {
	lsp              semanticapi.LSP
	wm               browserapi.WindowManager
	notify           browserapi.Notifications
	fs               workspaceapi.FileSystem
	scheduleNextTick func(func()) bool
	parser           syntaxapi.Parser
	cfg              HoverConfig
}

func (h *hoverHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	proceed, err := resolveCommandSymbol(ctx, &cmd, h.wm, h.fs, h.notify, h.scheduleNextTick, h.parser, func(m symbolMatch) {
		h.scheduleNextTick(func() {
			wsURI, err := LspToURI(m.URI)
			if err != nil {
				_, _ = h.notify.Notify(browserapi.LevelError, "hover: %s", err)
				return
			}
			if err := h.execute(context.Background(), wsURI, m.Pos); err != nil {
				_, _ = h.notify.Notify(browserapi.LevelError, "hover: %s", err)
			}
		})
	})
	if !proceed || err != nil {
		return err
	}
	return h.execute(ctx, cmd.URI, CoordToPos(cmd.Cursor.Content))
}

func (h *hoverHandler) execute(
	ctx context.Context, uri workspaceapi.URI, pos semanticapi.Position,
) error {
	params := semanticapi.HoverParams{
		TextDocument: TextDocID(uri),
		Position:     pos,
	}
	result, err := h.lsp.Hover(ctx, params)
	if err != nil {
		return err
	}
	if result == nil || result.Contents.Value == "" {
		return nil
	}
	if result.Contents.Kind == semanticapi.MarkupKindMarkdown {
		comp, mdErr := mdcomp.NewWithConfig(result.Contents.Value, h.cfg.MarkdownConfig)
		if mdErr != nil {
			return mdErr
		}
		mdh := mdhandler.New(comp, h.cfg.MarkdownHandlerOptions...)
		span := handler.NewSpan(mdh, component.SpanConfig{
			PadHorizontal:    2,
			ContentAlignment: component.AlignmentCentered,
		})
		var win browserapi.Window
		floating := browserapi.FuncFloatingHandler(span, func() error {
			defer h.wm.CloseWindow(win) //nolint:errcheck
			return mdh.Close()
		})
		win, err = h.wm.Floating(floating, browserapi.FloatingConfig{
			Alignment: component.AlignmentCentered,
		})
		return err
	}
	f := newHoverFloating(component.NewString(result.Contents.Value), h.wm)
	win, err := h.wm.Floating(f, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		return err
	}
	f.win = win
	return nil
}

func (h *hoverHandler) Complete(ctx context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return completeReferencedSymbol(ctx, h.parser)
}

func newHoverFloating(f component.Floating, wm browserapi.WindowManager) *hoverFloating {
	w, h := f.Dimensions()
	f.Resize(w, h)
	return &hoverFloating{Floating: f, wm: wm}
}

type hoverFloating struct {
	component.Floating
	wm  browserapi.WindowManager
	win browserapi.Window
}

// Handle dismisses the hover on any key event, including modifier-only
// keys. This is intentional: the hover tooltip is transient and should
// disappear on any keyboard interaction.
func (h *hoverFloating) Handle(ev term.Event) (bool, bool) {
	if ev.Type == term.EventKey {
		return true, true
	}
	return false, false
}

func (h *hoverFloating) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

func (h *hoverFloating) Selection() (string, bool) {
	return "", false
}

func (h *hoverFloating) Close() error {
	if h.win != nil {
		return h.wm.CloseWindow(h.win)
	}
	return nil
}
