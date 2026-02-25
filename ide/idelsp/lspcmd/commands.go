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
	"sort"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const cmdName = "lsp"

// Manual returns the textapi.CommandManual for the "lsp" command.
func Manual() textapi.CommandManual {
	return textapi.CommandManual{
		Name:     cmdName,
		Summary:  "Language Server Protocol commands",
		Synopsis: "<subcommand> [<args>...]",
		Commands: []textapi.CommandManual{
			{
				Name:    "format",
				Summary: "Formats the file or the selection if the cursor is currently selecting text",
			},
			{
				Name:    "hover",
				Summary: "Displays documentation, types, and signatures in a floating window",
			},
			{
				Name:    "complete",
				Summary: "Show completions at cursor",
			},
			{
				Name:    "implementation",
				Summary: "Find implementations of symbol",
			},
			{
				Name:    "references",
				Summary: "Find references to symbol",
			},
		},
	}
}

// AllConfig groups configuration for every subcommand registered by
// AllHandler.
type AllConfig struct {
	Hover          HoverConfig
	Complete       CompleteConfig
	Implementation ImplementationConfig
	References     ReferencesConfig

	// Interrupter signals the event loop to re-render after
	// asynchronous updates (e.g. completion results arriving).
	Interrupter term.Interrupter
}

// DefaultConfig returns an AllConfig where every subcommand uses its
// own default configuration.
func DefaultConfig() AllConfig {
	return AllConfig{
		Hover:          DefaultHoverConfig(),
		Complete:       DefaultCompleteConfig(),
		Implementation: DefaultImplementationConfig(),
		References:     DefaultReferencesConfig(),
		Interrupter:    term.NopInterrupter(),
	}
}

// AllHandler creates the textapi.CommandHandler for the "lsp" command
// by composing all individual subcommand handlers.
func AllHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, opener browserapi.ResourceOpener,
	notify browserapi.Notifications, fs workspaceapi.FileSystem,
	cfg AllConfig,
) (textapi.CommandHandler, error) {
	formatH, err := FormatHandler(lsp, editor)
	if err != nil {
		return nil, err
	}
	return &routerHandler{
		handlers: map[string]textapi.CommandHandler{
			"format":   formatH,
			"hover":    HoverHandler(lsp, wm, cfg.Hover),
			"complete": CompleteHandler(lsp, editor, wm, cfg.Complete, cfg.Interrupter),
			"implementation": ImplementationHandler(
				lsp, editor, wm, opener, notify, fs, cfg.Implementation,
			),
			"references": ReferencesHandler(
				lsp, editor, wm, opener, notify, fs, cfg.References,
			),
		},
	}, nil
}

var _ textapi.CommandHandler = (*routerHandler)(nil)

type routerHandler struct {
	handlers map[string]textapi.CommandHandler
}

func (r *routerHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	if cmd.Resource == nil {
		return nil
	}
	if cmd.Name != cmdName {
		return fmt.Errorf("unknown command: %s", cmd.Name)
	}
	if len(cmd.Args) == 0 {
		return fmt.Errorf("missing subcommand")
	}
	cmd.Name = cmd.Args[0]
	cmd.Args = cmd.Args[1:]
	h, ok := r.handlers[cmd.Name]
	if !ok {
		return fmt.Errorf("unknown lsp subcommand: %s", cmd.Name)
	}
	return h.HandleCommand(ctx, cmd)
}

func (r *routerHandler) Complete(
	ctx context.Context,
	cmd string, args []string,
) (iterator.Iterator[string], error) {
	if cmd != cmdName {
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
	if len(args) == 0 {
		return iterator.Empty[string](), nil
	}
	cmd = args[0]
	args = args[1:]
	if h, ok := r.handlers[cmd]; ok {
		return h.Complete(ctx, cmd, args)
	}
	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return iterator.FromSlice(names), nil
}
