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

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// FormatHandler creates a textapi.CommandHandler for the "format" subcommand.
// It subscribes to selection and cursor events to track active selections.
func FormatHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
) (textapi.CommandHandler, error) {
	evs := []textapi.EventType{textapi.EventTypeSelection, textapi.EventTypeCursor}
	sel := NewSelectionTracker()
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
	sel    *SelectionTracker
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
		TextDocument: TextDocID(cmd.URI),
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
	return ApplyEdits(ctx, ce, edits)
}

func (h *formatHandler) formatRange(
	ctx context.Context, cmd textapi.Command,
) error {
	selRange, ok := h.sel.Get(cmd.URI)
	if !ok {
		return h.formatFull(ctx, cmd)
	}
	params := semanticapi.DocumentRangeFormattingParams{
		TextDocument: TextDocID(cmd.URI),
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
	return ApplyEdits(ctx, ce, edits)
}
