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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func testURI(t *testing.T, path string) workspaceapi.URI {
	t.Helper()
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)
	return uri
}

func cursorEvent(
	uri workspaceapi.URI, line, char int,
) textapi.Event {
	return textapi.Event{
		Type: textapi.EventTypeCursor,
		URI:  uri,
		From: term.Coordinates{X: char, Y: line},
	}
}

func editEvent(uri workspaceapi.URI) textapi.Event {
	return textapi.Event{
		Type: textapi.EventTypeEdit,
		URI:  uri,
	}
}

func TestHighlightHandler_DebounceCoalesces(t *testing.T) {
	t.Parallel()

	uri := testURI(t, "/tmp/test.go")
	handler := &mockHandler{uri: uri}
	var calls atomic.Int32

	lsp := &mockLSP{
		documentHighlightFn: func(
			_ context.Context,
			params semanticapi.DocumentHighlightParams,
		) ([]semanticapi.DocumentHighlight, error) {
			calls.Add(1)
			return []semanticapi.DocumentHighlight{
				{
					Range: semanticapi.Range{
						Start: params.Position,
						End: semanticapi.Position{
							Line:      params.Position.Line,
							Character: params.Position.Character + 3,
						},
					},
					Kind: semanticapi.DocumentHighlightKindRead,
				},
			}, nil
		},
	}

	var setLocCalls atomic.Int32
	editor := &mockEditor{
		editorFn: func(_ workspaceapi.URI) (textapi.Handler, error) {
			return handler, nil
		},
		setLocationListFn: func(
			_ textapi.Handler,
			_ textapi.LocationPriority,
			_ string,
			_ textapi.LocationList,
		) error {
			setLocCalls.Add(1)
			return nil
		},
		subscribeEventsFn: func(
			_ []textapi.EventType,
			_ textapi.EventHandler,
		) error {
			return nil
		},
	}

	cfg := DefaultHighlightConfig()
	cfg.Delay = 50 * time.Millisecond
	h := &highlightHandler{
		lsp:    lsp,
		editor: editor,
		sched:  syncTick,
		delay:  cfg.Delay,
		cfg:    cfg,
	}

	ctx := context.Background()
	// Fire 5 rapid cursor events — only the last should trigger an RPC.
	for i := range 5 {
		h.Handle(ctx, cursorEvent(uri, 10, i))
	}

	// Wait for debounce + processing.
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(1), calls.Load(),
		"only one RPC should fire after debounce")
	assert.Equal(t, int32(1), setLocCalls.Load(),
		"only one SetLocationList call expected")
}

func TestHighlightHandler_SamePositionSkipped(t *testing.T) {
	t.Parallel()

	uri := testURI(t, "/tmp/test.go")
	var calls atomic.Int32

	lsp := &mockLSP{
		documentHighlightFn: func(
			_ context.Context,
			_ semanticapi.DocumentHighlightParams,
		) ([]semanticapi.DocumentHighlight, error) {
			calls.Add(1)
			return nil, nil
		},
	}

	editor := &mockEditor{
		editorFn: func(_ workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: uri}, nil
		},
		setLocationListFn: func(
			_ textapi.Handler,
			_ textapi.LocationPriority,
			_ string,
			_ textapi.LocationList,
		) error {
			return nil
		},
	}

	h := &highlightHandler{
		lsp:    lsp,
		editor: editor,
		sched:  syncTick,
		delay:  20 * time.Millisecond,
		cfg:    DefaultHighlightConfig(),
	}

	ctx := context.Background()
	h.Handle(ctx, cursorEvent(uri, 5, 10))
	time.Sleep(100 * time.Millisecond)

	// Second event at same position should be skipped entirely.
	h.Handle(ctx, cursorEvent(uri, 5, 10))
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(1), calls.Load(),
		"duplicate position should not trigger a second RPC")
}

func TestHighlightHandler_EditClearsHighlights(t *testing.T) {
	t.Parallel()

	uri := testURI(t, "/tmp/test.go")
	var rpcCalls atomic.Int32

	lsp := &mockLSP{
		documentHighlightFn: func(
			_ context.Context,
			_ semanticapi.DocumentHighlightParams,
		) ([]semanticapi.DocumentHighlight, error) {
			rpcCalls.Add(1)
			return nil, nil
		},
	}

	var mu sync.Mutex
	var lastID string
	var lastList textapi.LocationList

	editor := &mockEditor{
		editorFn: func(_ workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: uri}, nil
		},
		setLocationListFn: func(
			_ textapi.Handler,
			_ textapi.LocationPriority,
			id string,
			list textapi.LocationList,
		) error {
			mu.Lock()
			lastID = id
			lastList = list
			mu.Unlock()
			return nil
		},
	}

	h := &highlightHandler{
		lsp:    lsp,
		editor: editor,
		sched:  syncTick,
		delay:  50 * time.Millisecond,
		cfg:    DefaultHighlightConfig(),
	}

	ctx := context.Background()
	// Fire a cursor event, then immediately fire an edit before
	// the debounce fires.
	h.Handle(ctx, cursorEvent(uri, 5, 10))
	h.Handle(ctx, editEvent(uri))

	// Wait past the debounce window.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	gotID := lastID
	gotList := lastList
	mu.Unlock()

	assert.Equal(t, highlightLocationID, gotID,
		"edit should clear highlights via SetLocationList")
	assert.Nil(t, gotList,
		"edit should pass nil to clear the location list")
	assert.Equal(t, int32(0), rpcCalls.Load(),
		"debounced RPC should be cancelled by the edit")
}

func TestHighlightHandler_StaleResultDiscarded(t *testing.T) {
	t.Parallel()

	uri := testURI(t, "/tmp/test.go")

	// Block the first RPC until we send a second cursor event.
	firstCall := make(chan struct{})
	proceed := make(chan struct{})

	var callCount atomic.Int32

	lsp := &mockLSP{
		documentHighlightFn: func(
			ctx context.Context,
			params semanticapi.DocumentHighlightParams,
		) ([]semanticapi.DocumentHighlight, error) {
			n := callCount.Add(1)
			if n == 1 {
				close(firstCall)
				select {
				case <-proceed:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return []semanticapi.DocumentHighlight{
				{
					Range: semanticapi.Range{
						Start: params.Position,
						End: semanticapi.Position{
							Line:      params.Position.Line,
							Character: params.Position.Character + 3,
						},
					},
				},
			}, nil
		},
	}

	var mu sync.Mutex
	var locCalls int
	var lastList textapi.LocationList

	editor := &mockEditor{
		editorFn: func(_ workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: uri}, nil
		},
		setLocationListFn: func(
			_ textapi.Handler,
			_ textapi.LocationPriority,
			_ string,
			list textapi.LocationList,
		) error {
			mu.Lock()
			locCalls++
			lastList = list
			mu.Unlock()
			return nil
		},
	}

	h := &highlightHandler{
		lsp:    lsp,
		editor: editor,
		sched:  syncTick,
		delay:  10 * time.Millisecond,
		cfg:    DefaultHighlightConfig(),
	}

	ctx := context.Background()

	// First cursor event triggers RPC which blocks.
	h.Handle(ctx, cursorEvent(uri, 5, 10))

	// Wait for the first RPC to start.
	<-firstCall

	// Move cursor to a different position while first RPC is in-flight.
	// This cancels the first RPC's context and starts a new debounce.
	h.Handle(ctx, cursorEvent(uri, 10, 20))

	// Unblock the first RPC (it should see context cancelled).
	close(proceed)

	// Wait for the second debounce + RPC to complete.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	gotCalls := locCalls
	gotList := lastList
	mu.Unlock()

	assert.Equal(t, 1, gotCalls,
		"only the second (non-stale) result should be applied")
	assert.NotNil(t, gotList,
		"the applied result should have highlights")
}

func TestHighlightHandler_AppliesHighlightsCorrectly(t *testing.T) {
	t.Parallel()

	uri := testURI(t, "/tmp/test.go")
	handler := &mockHandler{uri: uri}

	lsp := &mockLSP{
		documentHighlightFn: func(
			_ context.Context,
			_ semanticapi.DocumentHighlightParams,
		) ([]semanticapi.DocumentHighlight, error) {
			return []semanticapi.DocumentHighlight{
				{
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 5, Character: 10},
						End:   semanticapi.Position{Line: 5, Character: 13},
					},
					Kind: semanticapi.DocumentHighlightKindRead,
				},
				{
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 8, Character: 4},
						End:   semanticapi.Position{Line: 8, Character: 7},
					},
					Kind: semanticapi.DocumentHighlightKindWrite,
				},
			}, nil
		},
	}

	var mu sync.Mutex
	var gotPriority textapi.LocationPriority
	var gotID string
	var gotList textapi.LocationList

	editor := &mockEditor{
		editorFn: func(_ workspaceapi.URI) (textapi.Handler, error) {
			return handler, nil
		},
		setLocationListFn: func(
			_ textapi.Handler,
			p textapi.LocationPriority,
			id string,
			list textapi.LocationList,
		) error {
			mu.Lock()
			gotPriority = p
			gotID = id
			gotList = list
			mu.Unlock()
			return nil
		},
	}

	h := &highlightHandler{
		lsp:    lsp,
		editor: editor,
		sched:  syncTick,
		delay:  10 * time.Millisecond,
		cfg:    DefaultHighlightConfig(),
	}

	ctx := context.Background()
	h.Handle(ctx, cursorEvent(uri, 5, 10))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, textapi.LocationPriorityInfo, gotPriority)
	assert.Equal(t, highlightLocationID, gotID)
	require.NotNil(t, gotList)

	loc, ok := gotList.Current()
	require.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 10, Y: 5}, loc.From)
	assert.Equal(t, term.Coordinates{X: 13, Y: 5}, loc.To)

	loc, ok = gotList.Next()
	require.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 4, Y: 8}, loc.From)
	assert.Equal(t, term.Coordinates{X: 7, Y: 8}, loc.To)
}

func TestSubscribeHighlight(t *testing.T) {
	t.Parallel()

	var subscribedTypes []textapi.EventType
	editor := &mockEditor{
		subscribeEventsFn: func(
			types []textapi.EventType, _ textapi.EventHandler,
		) error {
			subscribedTypes = types
			return nil
		},
	}

	err := SubscribeHighlight(
		&mockLSP{}, editor, syncTick,
		DefaultHighlightConfig(),
	)
	require.NoError(t, err)

	assert.Contains(t, subscribedTypes, textapi.EventTypeCursor)
	assert.Contains(t, subscribedTypes, textapi.EventTypeEdit)
}
