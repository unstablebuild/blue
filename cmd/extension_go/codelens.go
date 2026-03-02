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
	"math"

	"github.com/unstablebuild/blue/ide/idelsp/lspcmd"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func codeLensHandler(
	lsp semanticapi.LSP,
	notify browserapi.Notifications,
	commandName string,
) textapi.CommandHandler {
	return &codeLensCmd{lsp: lsp, notify: notify, commandName: commandName}
}

var _ textapi.CommandHandler = (*codeLensCmd)(nil)

type codeLensCmd struct {
	lsp         semanticapi.LSP
	notify      browserapi.Notifications
	commandName string
}

func (h *codeLensCmd) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	if cmd.Resource == nil {
		return nil
	}

	params := semanticapi.CodeLensParams{
		TextDocument: lspcmd.TextDocID(cmd.URI),
	}
	lenses, err := h.lsp.CodeLens(ctx, params)
	if err != nil {
		return fmt.Errorf("code lens: %w", err)
	}

	cursorLine := uint32(cmd.Cursor.Content.Y)
	var nearest *semanticapi.CodeLens
	minDist := uint32(math.MaxUint32)
	for i := range lenses {
		lens := &lenses[i]
		if lens.Command == nil || lens.Command.Command != h.commandName {
			continue
		}
		dist := absDiff(lens.Range.Start.Line, cursorLine)
		if dist < minDist {
			minDist = dist
			nearest = lens
		}
	}

	if nearest == nil || nearest.Command == nil {
		_, _ = h.notify.Notify(browserapi.LevelInfo, "No %s lens found", h.commandName)
		return nil
	}

	result, err := h.lsp.ExecuteCommand(ctx, semanticapi.ExecuteCommandParams{
		Command:   nearest.Command.Command,
		Arguments: nearest.Command.Arguments,
	})
	if err != nil {
		return fmt.Errorf("execute %s: %w", h.commandName, err)
	}

	if result != "" {
		_, _ = h.notify.Notify(browserapi.LevelInfo, "%s", result)
	} else {
		_, _ = h.notify.Notify(browserapi.LevelInfo, "Executed: %s", nearest.Command.Title)
	}
	return nil
}

func (h *codeLensCmd) Complete(
	_ context.Context, _ string, _ []string,
) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
