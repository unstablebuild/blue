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
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// FormatHandler creates a textapi.CommandHandler for the "format" subcommand.
// It subscribes to selection and cursor events to track active selections.
func FormatHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
) (textapi.CommandHandler, error) {
	evs := []textapi.EventType{textapi.EventTypeSelection, textapi.EventTypeCursor}
	sel := newSelectionTracker()
	err := editor.SubscribeEvents(evs, sel)
	if err != nil {
		return nil, err
	}
	return &formatHandler{lsp: lsp, editor: editor, sel: sel}, nil
}

var _ textapi.CommandHandler = (*formatHandler)(nil)

type formatHandler struct {
	lsp    semanticapi.LSP
	editor textapi.Editor
	sel    *selectionTracker
}

func (h *formatHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	if cmd.Resource == nil {
		return nil
	}
	_, hasSelection := cmd.Resource.Selection()
	if hasSelection {
		return h.formatRange(ctx, cmd)
	}
	return h.formatFull(ctx, cmd)
}

func (h *formatHandler) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

func (h *formatHandler) formatFull(
	ctx context.Context, cmd textapi.Command,
) error {
	params := semanticapi.DocumentFormattingParams{
		TextDocument: textDocID(cmd.URI),
		Options: semanticapi.FormattingOptions{
			TabSize:      4,
			InsertSpaces: false,
		},
	}
	edits, err := h.lsp.Formatting(ctx, params)
	if err != nil {
		return err
	}
	if len(edits) == 0 {
		return nil
	}
	ce := h.editor.CellEditor(cmd.Resource)
	return applyEdits(ctx, ce, edits)
}

func (h *formatHandler) formatRange(
	ctx context.Context, cmd textapi.Command,
) error {
	selRange, ok := h.sel.get(cmd.URI)
	if !ok {
		return h.formatFull(ctx, cmd)
	}
	params := semanticapi.DocumentRangeFormattingParams{
		TextDocument: textDocID(cmd.URI),
		Range:        selRange,
		Options: semanticapi.FormattingOptions{
			TabSize:      4,
			InsertSpaces: false,
		},
	}
	edits, err := h.lsp.RangeFormatting(ctx, params)
	if err != nil {
		return err
	}
	if len(edits) == 0 {
		return nil
	}
	ce := h.editor.CellEditor(cmd.Resource)
	return applyEdits(ctx, ce, edits)
}

var _ textapi.EventHandler = (*selectionTracker)(nil)

type selectionTracker struct {
	mu   sync.Mutex
	sels map[string]semanticapi.Range
}

func newSelectionTracker() *selectionTracker {
	return &selectionTracker{
		sels: make(
			map[string]semanticapi.Range,
		),
	}
}

func (s *selectionTracker) Handle(
	_ context.Context, ev textapi.Event,
) bool {
	uri := uriToLSP(ev.URI)

	s.mu.Lock()
	defer s.mu.Unlock()

	switch ev.Type {
	case textapi.EventTypeSelection:
		s.sels[uri] = semanticapi.Range{Start: coordToPos(ev.Start), End: coordToPos(ev.End)}
	case textapi.EventTypeCursor:
		delete(s.sels, uri)
	}
	return false
}

func (s *selectionTracker) get(
	uri workspaceapi.URI,
) (semanticapi.Range, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.sels[uriToLSP(uri)]
	return r, ok
}
