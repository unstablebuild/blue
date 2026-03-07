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

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// ImplementationConfig configures the "implementation" subcommand.
type ImplementationConfig struct {
	ListConfig LocationsConfig
}

// DefaultImplementationConfig returns an ImplementationConfig with sensible defaults.
func DefaultImplementationConfig() ImplementationConfig {
	return ImplementationConfig{
		ListConfig: DefaultLocationsConfig(),
	}
}

// ImplementationHandler creates a textapi.CommandHandler that finds
// implementations of the symbol at the cursor position and displays them in a
// floating window with a file preview.
func ImplementationHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, opener browserapi.ResourceOpener,
	notify browserapi.Notifications, fs workspaceapi.FileSystem,
	rootURI workspaceapi.URI, scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser, cfg ImplementationConfig,
) textapi.CommandHandler {
	return &implementationHandler{
		lsp: lsp, editor: editor, wm: wm, opener: opener,
		notify: notify, fs: fs, rootURI: rootURI,
		scheduleNextTick: scheduleNextTick, parser: parser, cfg: cfg,
	}
}

type implementationHandler struct {
	lsp              semanticapi.LSP
	editor           textapi.Editor
	wm               browserapi.WindowManager
	opener           browserapi.ResourceOpener
	notify           browserapi.Notifications
	fs               workspaceapi.FileSystem
	rootURI          workspaceapi.URI
	scheduleNextTick func(func()) bool
	parser           syntaxapi.Parser
	cfg              ImplementationConfig
}

func (h *implementationHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	proceed, err := resolveCommandSymbol(ctx, &cmd, h.wm, h.fs, h.scheduleNextTick, h.parser, func(m symbolMatch) {
		h.scheduleNextTick(func() {
			wsURI, err := LspToURI(m.URI)
			if err != nil {
				_, _ = h.notify.Notify(browserapi.LevelError, "implementation: %s", err)
				return
			}
			if err := h.execute(context.Background(), wsURI, m.Pos); err != nil {
				_, _ = h.notify.Notify(browserapi.LevelError, "implementation: %s", err)
			}
		})
	})
	if !proceed || err != nil {
		return err
	}
	return h.execute(ctx, cmd.URI, CoordToPos(cmd.Cursor.Content))
}

func (h *implementationHandler) execute(
	ctx context.Context, uri workspaceapi.URI, pos semanticapi.Position,
) error {
	params := semanticapi.ImplementationParams{
		TextDocument: TextDocID(uri),
		Position:     pos,
	}
	result, err := h.lsp.Implementation(ctx, params)
	if err != nil {
		return err
	}
	entries := locationsFromResult(result)
	if len(entries) == 0 {
		return nil
	}
	entries = enrichEntries(entries, h.rootURI)
	if len(entries) == 1 {
		navigateTo(entries[0], h.opener, h.wm, h.editor, h.notify, h.scheduleNextTick)
		return nil
	}
	handler := newLocationsFloatingHandler(
		entries, h.opener, h.wm, h.editor, h.notify, h.fs,
		h.scheduleNextTick, h.parser, h.cfg.ListConfig,
	)
	win, err := h.wm.Floating(handler, browserapi.FloatingConfig{Alignment: component.AlignmentCentered})
	if err != nil {
		return err
	}
	handler.win = win
	return nil
}

func (h *implementationHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	return completeReferencedSymbol(ctx, h.parser)
}
