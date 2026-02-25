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
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
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
	lsp semanticapi.LSP, wm browserapi.WindowManager, cfg HoverConfig,
) textapi.CommandHandler {
	return &hoverHandler{lsp: lsp, wm: wm, cfg: cfg}
}

var (
	_ textapi.CommandHandler = (*hoverHandler)(nil)
	_ browserapi.Floating    = (*hoverFloating)(nil)
)

type hoverHandler struct {
	lsp semanticapi.LSP
	wm  browserapi.WindowManager
	cfg HoverConfig
}

func (h *hoverHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if cmd.Resource == nil {
		return nil
	}
	params := semanticapi.HoverParams{
		TextDocument: textDocID(cmd.URI),
		Position:     coordToPos(cmd.Cursor.Content),
	}
	result, err := h.lsp.Hover(ctx, params)
	if err != nil {
		return err
	}
	if result == nil || result.Contents.Value == "" {
		return nil
	}
	var floating browserapi.Floating
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
		floating = browserapi.FuncFloatingHandler(span, mdh.Close)
	} else {
		floating = newHoverFloating(component.NewString(result.Contents.Value))
	}
	_, err = h.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	return err
}

func (h *hoverHandler) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

func newHoverFloating(f component.Floating) *hoverFloating {
	w, h := f.Dimensions()
	f.Resize(w, h)
	return &hoverFloating{Floating: f}
}

type hoverFloating struct {
	component.Floating
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
	return nil
}
