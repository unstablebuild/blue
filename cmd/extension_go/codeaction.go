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

package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/unstablebuild/blue/ide/idelsp/lspcmd"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func codeActionHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	notify browserapi.Notifications,
	wm browserapi.WindowManager,
	sel *lspcmd.SelectionTracker,
	kind semanticapi.CodeActionKind,
	noActionHint string,
) textapi.CommandHandler {
	return &codeActionCmd{lsp: lsp, editor: editor, notify: notify,
		wm: wm, sel: sel, kind: kind, noActionHint: noActionHint}
}

var _ textapi.CommandHandler = (*codeActionCmd)(nil)

type codeActionCmd struct {
	lsp          semanticapi.LSP
	editor       textapi.Editor
	notify       browserapi.Notifications
	wm           browserapi.WindowManager
	sel          *lspcmd.SelectionTracker
	kind         semanticapi.CodeActionKind
	noActionHint string
}

func (h *codeActionCmd) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	if cmd.Resource == nil {
		return nil
	}

	rng, ok := h.sel.Get(cmd.URI)
	slog.Debug("handle code action command", "uri", cmd.URI, "selection-range", rng, "selection", ok)
	if !ok {
		cursorPos := lspcmd.CoordToPos(cmd.Cursor.Content)
		rng = semanticapi.Range{Start: cursorPos, End: cursorPos}
	} else {
		rng = lspcmd.ClampRange(rng, h.editor, cmd.Resource)
		slog.Debug("clamped selection range", "uri", cmd.URI, "selection-range", rng)
	}

	params := semanticapi.CodeActionParams{
		TextDocument: lspcmd.TextDocID(cmd.URI),
		Range:        rng,
		Context: semanticapi.CodeActionContext{
			Only:        []semanticapi.CodeActionKind{h.kind},
			TriggerKind: semanticapi.CodeActionTriggerKindInvoked,
		},
	}

	results, err := h.lsp.CodeAction(ctx, params)
	if err != nil {
		return fmt.Errorf("code action: %w", err)
	}

	var actions []semanticapi.CodeAction
	for _, r := range results {
		if r.CodeAction == nil {
			continue
		}
		if strings.HasPrefix(string(r.CodeAction.Kind), string(h.kind)) {
			actions = append(actions, *r.CodeAction)
		}
	}

	if len(actions) == 0 {
		_, _ = h.notify.Notify(browserapi.LevelInfo, h.noActionHint)
		return nil
	}

	if len(actions) == 1 {
		return h.applyAction(ctx, actions[0])
	}

	ch := make(chan int, 1)
	picker := newCodeActionPicker(actions, ch)
	_, err = h.wm.Floating(picker, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		return fmt.Errorf("show picker: %w", err)
	}

	select {
	case idx := <-ch:
		if idx < 0 {
			return nil
		}
		return h.applyAction(ctx, actions[idx])
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *codeActionCmd) applyAction(ctx context.Context, action semanticapi.CodeAction) error {
	if action.Edit != nil {
		err := lspcmd.ApplyWorkspaceEdit(ctx, h.editor, nil, action.Edit)
		if err != nil {
			return err
		}
	}
	if action.Command != nil {
		_, err := h.lsp.ExecuteCommand(ctx, semanticapi.ExecuteCommandParams{
			Command:   action.Command.Command,
			Arguments: action.Command.Arguments,
		})
		if err != nil {
			return fmt.Errorf("execute command %s: %w", action.Command.Command, err)
		}
		_, _ = h.notify.Notify(browserapi.LevelInfo, "Executed: %s", action.Title)
	}
	return nil
}

func (h *codeActionCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// codeActionPicker is a floating handler that presents a list of code
// actions for the user to choose from using keyboard navigation.
type codeActionPicker struct {
	actions       []semanticapi.CodeAction
	list          *component.FocusList
	ch            chan<- int
	idealW, idealH int
}

const maxPickerHeight = 15

var _ browserapi.Floating = (*codeActionPicker)(nil)

func newCodeActionPicker(
	actions []semanticapi.CodeAction, ch chan<- int,
) *codeActionPicker {
	list := component.NewFocusList()
	maxW := 0
	for _, a := range actions {
		list.PushBack(component.NewResponsiveString(
			a.Title, component.StringResponsiveConfig{},
		))
		if w := utf8.RuneCountInString(a.Title); w > maxW {
			maxW = w
		}
	}
	idealH := min(len(actions), maxPickerHeight)
	return &codeActionPicker{
		actions: actions,
		list:    list,
		ch:      ch,
		idealW:  maxW,
		idealH:  idealH,
	}
}

func (p *codeActionPicker) Resize(width, height int) {
	p.list.Resize(width, height)
}

func (p *codeActionPicker) Draw(w term.Writer) {
	p.list.Draw(w)
}

func (p *codeActionPicker) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return false, false
	}
	switch ev.Key {
	case term.KeyEsc:
		p.ch <- -1
		return true, true
	case term.KeyEnter:
		idx := p.list.FocusOffset()
		if idx >= 0 && idx < len(p.actions) {
			p.ch <- idx
		} else {
			p.ch <- -1
		}
		return true, true
	case term.KeyArrowUp:
		p.list.FocusUp()
		return false, true
	case term.KeyArrowDown:
		p.list.FocusDown()
		return false, true
	}
	if ev.Mod == term.ModCtrl {
		switch ev.Ch {
		case 'j':
			p.list.FocusDown()
			return false, true
		case 'k':
			p.list.FocusUp()
			return false, true
		case 'c':
			p.ch <- -1
			return true, true
		}
	}
	return false, false
}

func (p *codeActionPicker) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

func (p *codeActionPicker) Selection() (string, bool) {
	return "", false
}

func (p *codeActionPicker) Close() error {
	select {
	case p.ch <- -1:
	default:
	}
	return nil
}

func (p *codeActionPicker) Dimensions() (int, int) {
	return p.idealW, p.idealH
}
