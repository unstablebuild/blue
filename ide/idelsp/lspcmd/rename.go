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
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/inputbox"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// RenameHandler creates a textapi.CommandHandler for the
// "rename" subcommand. It shows an inputbox under the cursor,
// pre-filled with the current symbol name, and applies the
// workspace edit returned by the language server.
func RenameHandler(
	lsp semanticapi.LSP, editor textapi.Editor, wm browserapi.WindowManager,
	opener browserapi.ResourceOpener,
) textapi.CommandHandler {
	return &renameHandler{
		lsp:    lsp,
		editor: editor,
		wm:     wm,
		opener: opener,
	}
}

var _ textapi.CommandHandler = (*renameHandler)(nil)

type renameHandler struct {
	lsp    semanticapi.LSP
	editor textapi.Editor
	wm     browserapi.WindowManager
	opener browserapi.ResourceOpener
}

func (h *renameHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if cmd.Resource == nil {
		return nil
	}
	pos := CoordToPos(cmd.Cursor.Content)

	prep, err := h.lsp.PrepareRename(ctx, semanticapi.PrepareRenameParams{
		TextDocument: TextDocID(cmd.URI),
		Position:     pos,
	})
	if err != nil {
		return fmt.Errorf("prepare rename: %w", err)
	}
	if prep == nil {
		return fmt.Errorf("rename not available at cursor position")
	}

	ib := inputbox.New(
		inputbox.WithPrompt(""),
		inputbox.WithText(prep.Placeholder),
	)

	ctx, cancel := context.WithCancel(context.Background())
	floating := &renameFloatingHandler{
		ib:       ib,
		lsp:      h.lsp,
		editor:   h.editor,
		opener:   h.opener,
		uri:      cmd.URI,
		position: pos,
		ctx:      ctx,
		cancel:   cancel,
	}

	_, err = h.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentLeft | component.AlignmentTop,
		Offset: term.Coordinates{
			X: cmd.Cursor.Window.X + 1,
			Y: cmd.Cursor.Window.Y + 2,
		},
	})
	return err
}

func (h *renameHandler) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// renameFloatingHandler wraps an inputbox.Handler to
// implement browserapi.Floating. On confirmation it calls
// LSP rename and applies the returned workspace edit.
var _ browserapi.Floating = (*renameFloatingHandler)(nil)

type renameFloatingHandler struct {
	ib       *inputbox.Handler
	lsp      semanticapi.LSP
	editor   textapi.Editor
	opener   browserapi.ResourceOpener
	uri      workspaceapi.URI
	position semanticapi.Position
	ctx      context.Context
	cancel   context.CancelFunc
}

func (r *renameFloatingHandler) Handle(ev term.Event) (bool, bool) {
	isEsc := ev.Type == term.EventKey && ev.Key == term.KeyEsc
	exit, handled := r.ib.Handle(ev)
	if !exit {
		return false, handled
	}
	if isEsc {
		return true, true
	}

	newName := r.ib.Text()
	if newName == "" {
		return true, true
	}

	edit, err := r.lsp.Rename(r.ctx, semanticapi.RenameParams{
		TextDocument: TextDocID(r.uri),
		Position:     r.position,
		NewName:      newName,
	})
	if err != nil {
		slog.Warn("rename", "err", err)
		return true, true
	}
	if edit == nil {
		return true, true
	}

	err = ApplyWorkspaceEdit(r.ctx, r.editor, r.opener, edit)
	if err != nil {
		slog.Warn("rename apply", "err", err)
		return true, true
	}
	return true, true
}

func (r *renameFloatingHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return r.ib.Cursor()
}

func (r *renameFloatingHandler) Selection() (string, bool) {
	return r.ib.Selection()
}

func (r *renameFloatingHandler) Resize(width, height int) {
	r.ib.Resize(width, height)
}

func (r *renameFloatingHandler) Draw(w term.Writer) {
	r.ib.Draw(w)
}

func (r *renameFloatingHandler) Dimensions() (int, int) {
	textW := utf8.RuneCountInString(r.ib.Text())
	return max(textW+2, 30), 1
}

func (r *renameFloatingHandler) Close() error {
	r.cancel()
	return nil
}

// ApplyWorkspaceEdit applies a WorkspaceEdit by routing
// each file's text edits through the editor API.
// Per the LSP spec, DocumentChanges is preferred over
// Changes when both are present.
func ApplyWorkspaceEdit(
	ctx context.Context, editor textapi.Editor,
	opener browserapi.ResourceOpener,
	edit *semanticapi.WorkspaceEdit,
) error {
	if len(edit.DocumentChanges) > 0 {
		for _, dc := range edit.DocumentChanges {
			switch {
			case dc.TextDocumentEdit != nil:
				err := ApplyEditsForURI(ctx, editor, opener,
					dc.TextDocumentEdit.TextDocument.URI, dc.TextDocumentEdit.Edits,
				)
				if err != nil {
					return err
				}
			case dc.CreateFile != nil:
				slog.Warn("rename: create file not supported", "uri", dc.CreateFile.URI)
			case dc.RenameFile != nil:
				slog.Warn("rename: file rename not supported",
					"old", dc.RenameFile.OldURI,
					"new", dc.RenameFile.NewURI)
			case dc.DeleteFile != nil:
				slog.Warn("rename: delete file not supported",					"uri", dc.DeleteFile.URI)
			}
		}
		return nil
	}
	for uriStr, edits := range edit.Changes {
		err := ApplyEditsForURI(ctx, editor, opener, uriStr, edits)
		if err != nil {
			return err
		}
	}
	return nil
}


func ApplyEditsForURI(
	ctx context.Context, editor textapi.Editor,
	opener browserapi.ResourceOpener,
	uriStr string, edits []semanticapi.TextEdit,
) error {
	uri, err := LspToURI(uriStr)
	if err != nil {
		return fmt.Errorf("parse URI %s: %w", uriStr, err)
	}
	handler, err := editor.Editor(uri)
	if err != nil && opener != nil {
		if _, openErr := opener.Open(uri); openErr == nil {
			handler, err = editor.Editor(uri)
		}
	}
	if err != nil {
		return fmt.Errorf("editor for %s: %w", uri.Name(), err)
	}
	ce := editor.CellEditor(handler)
	if err := ApplyEdits(ctx, ce, edits); err != nil {
		return fmt.Errorf("apply edits for %s: %w", uri.Name(), err)
	}
	return nil
}
