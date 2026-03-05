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
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// ReferencesConfig configures the "references" subcommand.
type ReferencesConfig struct {
	ListConfig LocationsConfig
}

// DefaultReferencesConfig returns a ReferencesConfig with sensible defaults.
func DefaultReferencesConfig() ReferencesConfig {
	return ReferencesConfig{
		ListConfig: DefaultLocationsConfig(),
	}
}

// ReferencesHandler creates a textapi.CommandHandler that finds all references
// to the symbol at the cursor position and displays them in a floating window
// with a file preview.
func ReferencesHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, opener browserapi.ResourceOpener,
	notify browserapi.Notifications, fs workspaceapi.FileSystem,
	rootURI workspaceapi.URI, scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser, cfg ReferencesConfig,
) textapi.CommandHandler {
	return &referencesHandler{
		lsp: lsp, editor: editor, wm: wm, opener: opener,
		notify: notify, fs: fs, rootURI: rootURI,
		scheduleNextTick: scheduleNextTick, parser: parser, cfg: cfg,
	}
}

type referencesHandler struct {
	lsp              semanticapi.LSP
	editor           textapi.Editor
	wm               browserapi.WindowManager
	opener           browserapi.ResourceOpener
	notify           browserapi.Notifications
	fs               workspaceapi.FileSystem
	rootURI          workspaceapi.URI
	scheduleNextTick func(func()) bool
	parser           syntaxapi.Parser
	cfg              ReferencesConfig
}

func (h *referencesHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if len(cmd.Args) > 0 {
		uri, pos, err := ResolveSymbol(ctx, h.lsp, strings.Join(cmd.Args, " "))
		if err != nil {
			return err
		}
		wsURI, err := LspToURI(uri)
		if err != nil {
			return err
		}
		cmd.URI = wsURI
		cmd.Cursor.Content = PosToCoord(pos)
	} else if cmd.Resource == nil {
		return nil
	}
	params := semanticapi.ReferenceParams{
		TextDocument: TextDocID(cmd.URI),
		Position:     CoordToPos(cmd.Cursor.Content),
		Context:      semanticapi.ReferenceContext{IncludeDeclaration: true},
	}
	locs, err := h.lsp.References(ctx, params)
	if err != nil {
		return err
	}
	if len(locs) == 0 {
		return nil
	}
	entries := make([]locationEntry, len(locs))
	for i, loc := range locs {
		entries[i] = locationFromLoc(loc)
	}
	entries = enrichEntries(entries, h.rootURI)
	if len(entries) == 1 {
		navigateTo(entries[0], h.opener, h.wm, h.editor, h.notify, h.scheduleNextTick)
		return nil
	}
	handler := newLocationsFloatingHandler(
		entries, h.opener, h.wm, h.editor, h.notify,
		h.fs, h.scheduleNextTick, h.parser, h.cfg.ListConfig,
	)
	_, err = h.wm.Floating(handler, browserapi.FloatingConfig{Alignment: component.AlignmentCentered})
	return err
}

func (h *referencesHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	return CompleteSymbol(ctx, h.lsp, args)
}
