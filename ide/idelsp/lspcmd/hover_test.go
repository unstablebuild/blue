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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestHoverFloatingHandle(t *testing.T) {
	t.Parallel()
	f := newHoverFloating(component.NewString("test content"))

	exit, handled := f.Handle(term.Event{Type: term.EventKey})
	assert.True(t, exit, "key event should exit")
	assert.True(t, handled, "key event should be handled")

	exit, handled = f.Handle(term.Event{Type: term.EventMouse})
	assert.False(t, exit, "non-key event should not exit")
	assert.False(t, handled, "non-key event should not be handled")
}

func TestHoverHandlerSymbolName(t *testing.T) {
	t.Parallel()

	var hoveredPos semanticapi.Position
	lsp := &mockLSP{
		workspaceSymbolFn: func(_ context.Context, _ semanticapi.WorkspaceSymbolParams) ([]semanticapi.SymbolInformation, error) {
			return []semanticapi.SymbolInformation{
				{
					Name: "MyType",
					Location: semanticapi.Location{
						URI: "file:///project/types.go",
						Range: semanticapi.Range{
							Start: semanticapi.Position{Line: 7, Character: 5},
						},
					},
				},
			}, nil
		},
	}
	lsp.stubLSP = stubLSP{}
	// Override Hover to capture the position and return content.
	hoverFn := func(_ context.Context, params semanticapi.HoverParams) (*semanticapi.Hover, error) {
		hoveredPos = params.Position
		return &semanticapi.Hover{
			Contents: semanticapi.MarkupContent{
				Kind:  semanticapi.MarkupKindPlainText,
				Value: "type MyType struct{}",
			},
		}, nil
	}

	// We need a custom mock since mockLSP doesn't have hoverFn.
	// Instead, use a wrapper that intercepts Hover.
	wrapper := &hoverMockLSP{mockLSP: lsp, hoverFn: hoverFn}

	var floated bool
	wm := &mockWindowManager{
		floatingFn: func(_ browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
			floated = true
			return nil, nil
		},
	}

	h := HoverHandler(wrapper, wm, &mockNotifications{}, &mockFileSystem{}, syncTick, nil, DefaultHoverConfig())

	// No resource, but args provided — should still work.
	cmd := textapi.Command{
		Name: "hover",
		Args: []string{"MyType"},
	}
	err := h.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	assert.True(t, floated, "hover floating should be shown")
	assert.Equal(t, semanticapi.Position{Line: 7, Character: 5}, hoveredPos)
}

// hoverMockLSP wraps mockLSP and overrides Hover.
type hoverMockLSP struct {
	*mockLSP
	hoverFn func(context.Context, semanticapi.HoverParams) (*semanticapi.Hover, error)
}

func (m *hoverMockLSP) Hover(ctx context.Context, params semanticapi.HoverParams) (*semanticapi.Hover, error) {
	if m.hoverFn != nil {
		return m.hoverFn(ctx, params)
	}
	return nil, nil
}
