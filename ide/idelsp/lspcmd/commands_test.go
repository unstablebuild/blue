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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	idelsp "github.com/unstablebuild/blue/ide/idelsp"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestE2ECommands(t *testing.T) {
	t.Parallel()
	goplsBin := findGopls(t)
	tmpDir := setupTestWorkspace(t, "../testdata")

	rootURI, err := workspaceapi.ParseURI(
		"file://" + tmpDir,
	)
	require.NoError(t, err)

	mainPath := filepath.Join(tmpDir, "main.go")
	mainContent, err := os.ReadFile(mainPath)
	require.NoError(t, err)

	utilPath := filepath.Join(tmpDir, "util.go")
	utilContent, err := os.ReadFile(utilPath)
	require.NoError(t, err)

	testPath := filepath.Join(tmpDir, "main_test.go")
	testContent, err := os.ReadFile(testPath)
	require.NoError(t, err)

	mainURI := "file://" + mainPath
	mainWSURI, err := workspaceapi.ParseURI(mainURI)
	require.NoError(t, err)

	utilURI := "file://" + utilPath
	testFileURI := "file://" + testPath

	scheme := newTestScheme()

	readyCh := make(chan struct{})
	var ready sync.Once
	callback := &e2eCallback{
		onShowMessage: func(params semanticapi.ShowMessageParams) {
			if strings.Contains(params.Message, "Finished loading packages") {
				ready.Do(func() { close(readyCh) })
			}
		},
		onProgress: readyOnProgress(&ready, readyCh),
	}

	mgr := idelsp.New(
		rootURI, scheme, scheme, &stubPkgManager{bin: goplsBin},
		nil, nil, idelsp.Config{Callback: callback, MaxRetries: 1},
	)

	ctx := context.Background()

	ev := textapi.Event{
		Type:    textapi.EventTypeOpen,
		URI:     mainWSURI,
		Content: string(mainContent),
	}
	assert.False(t, mgr.Handle(ctx, ev))

	utilWSURI, err := workspaceapi.ParseURI(utilURI)
	require.NoError(t, err)
	mgr.Handle(ctx, textapi.Event{
		Type:    textapi.EventTypeOpen,
		URI:     utilWSURI,
		Content: string(utilContent),
	})

	testWSURI, err := workspaceapi.ParseURI(testFileURI)
	require.NoError(t, err)
	mgr.Handle(ctx, textapi.Event{
		Type:    textapi.EventTypeOpen,
		URI:     testWSURI,
		Content: string(testContent),
	})

	waitReady(t, readyCh)
	t.Cleanup(func() { _ = mgr.Close() })

	editor := &mockEditor{
		editorFn: func(
			uri workspaceapi.URI,
		) (textapi.Handler, error) {
			return &mockHandler{uri: uri}, nil
		},
	}
	wm := &mockWindowManager{}
	opener := &mockResourceOpener{}
	notify := &mockNotifications{}
	fs := &mockFileSystem{
		openFileFn: func(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
			return os.OpenFile(path, flag, mode)
		},
	}
	cfg := DefaultConfig()
	cfg.RootURI = rootURI
	cfg.ScheduleNextTick = syncTick
	router, err := AllHandler(mgr, editor, wm, opener, notify, fs, cfg)
	require.NoError(t, err)

	makeCmd := func(name string, args []string, line, char int) textapi.Command {
		cmd := textapi.Command{
			Name:     "lsp",
			Args:     append([]string{name}, args...),
			URI:      mainWSURI,
			Resource: &mockHandler{uri: mainWSURI},
		}
		cmd.Cursor.Content = term.Coordinates{X: char, Y: line}
		cmd.Cursor.Window = term.Coordinates{X: char, Y: line}
		return cmd
	}

	t.Run("references", func(t *testing.T) {
		var floatingHandler browserapi.Floating
		wm.floatingFn = func(
			h browserapi.Floating,
			_ browserapi.FloatingConfig,
		) (browserapi.Window, error) {
			floatingHandler = h
			return nil, nil
		}
		defer func() {
			wm.floatingFn = nil
		}()

		// Add is at line 38 (0-indexed), col 5.
		cmd := makeCmd("references", nil, 38, 5)
		err := router.HandleCommand(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, floatingHandler)

		lh, ok := floatingHandler.(*locationsFloatingHandler)
		require.True(t, ok)
		assert.GreaterOrEqual(
			t, len(lh.entries), 3,
			"expected at least 3 references to Add",
		)
	})

	t.Run("implementation", func(t *testing.T) {
		// Speaker has a single implementation (Robot), so the
		// handler navigates directly instead of showing a picker.
		var navigated bool
		editor.setCursorFn = func(_ textapi.Handler, _ term.Coordinates) error {
			navigated = true
			return nil
		}
		defer func() {
			editor.setCursorFn = nil
		}()

		// Speaker is at line 50 (0-indexed), col 5.
		cmd := makeCmd("implementation", nil, 50, 5)
		err := router.HandleCommand(ctx, cmd)
		require.NoError(t, err)
		assert.True(t, navigated, "expected direct navigation to single implementation")
	})

	t.Run("format", func(t *testing.T) {
		var (
			cellEditorCalled bool
			cellEditorH      textapi.Handler

			editCalled bool
			editStart  term.Coordinates
			editEnd    term.Coordinates
			editText   string
		)

		editor.cellEditorFn = func(h textapi.Handler) textapi.CellEditor {
			cellEditorCalled = true
			cellEditorH = h
			return &mockCellEditor{
				editFn: func(_ context.Context, s, e term.Coordinates, text string) (
					term.Coordinates, term.Coordinates, string, error,
				) {
					editCalled = true
					editStart = s
					editEnd = e
					editText = text
					return term.Coordinates{}, term.Coordinates{}, "", nil
				},
			}
		}
		defer func() {
			editor.cellEditorFn = nil
		}()

		cmd := makeCmd("format", nil, 0, 0)
		err := router.HandleCommand(ctx, cmd)
		require.NoError(t, err)

		assert.True(t, cellEditorCalled, "CellEditor must be called")
		assert.Equal(t, cmd.Resource, cellEditorH, "CellEditor called with wrong handler")
		assert.True(t, editCalled, "Edit must be called")
		assert.Equal(t, term.Coordinates{X: 0, Y: 62}, editStart)
		assert.Equal(t, term.Coordinates{X: 1, Y: 62}, editEnd)
		assert.Empty(t, editText)
	})

	// Hover tests use line 33, col 20 of testdata/main.go which
	// is inside the "Greet" method name on:
	//   func (g *Greeter) Greet() string {
	// The two sub-cases are not table-driven because "above cursor"
	// additionally verifies rendered content while "below cursor"
	// only checks offset positioning.
	t.Run("hover", func(t *testing.T) {
		t.Run("above cursor", func(t *testing.T) {
			var gotFloating browserapi.Floating

			wm.floatingFn = func(h browserapi.Floating, cfg browserapi.FloatingConfig) (
				browserapi.Window, error,
			) {
				gotFloating = h
				return nil, nil
			}
			defer func() { wm.floatingFn = nil }()

			cmd := makeCmd("hover", nil, 33, 20)
			err := router.HandleCommand(ctx, cmd)
			require.NoError(t, err)
			require.NotNil(t, gotFloating, "Floating must be called")

			w, h := gotFloating.Dimensions()
			gotFloating.Resize(w, h)
			sw := term.NewStringWriter(w, h)
			gotFloating.Draw(sw)
			require.NoError(t, sw.Flush())
			rendered := sw.String()

			assert.Contains(t, rendered, "func (g *Greeter) Greet() string")
		})

		t.Run("below cursor", func(t *testing.T) {
			wm.floatingFn = func(_ browserapi.Floating, cfg browserapi.FloatingConfig) (
				browserapi.Window, error,
			) {
				return nil, nil
			}
			defer func() { wm.floatingFn = nil }()

			cmd := makeCmd("hover", nil, 33, 20)
			cmd.Cursor.Window.Y = 0
			err := router.HandleCommand(ctx, cmd)
			require.NoError(t, err)
		})
	})
}

func TestE2ESignatureHelpAutoTrigger(t *testing.T) {
	t.Parallel()
	goplsBin := findGopls(t)
	tmpDir := setupTestWorkspace(t, "../testdata")

	rootURI, err := workspaceapi.ParseURI("file://" + tmpDir)
	require.NoError(t, err)

	mainPath := filepath.Join(tmpDir, "main.go")
	mainContent, err := os.ReadFile(mainPath)
	require.NoError(t, err)

	mainURI := "file://" + mainPath
	mainWSURI, err := workspaceapi.ParseURI(mainURI)
	require.NoError(t, err)

	scheme := newTestScheme()

	readyCh := make(chan struct{})
	var ready sync.Once
	callback := &e2eCallback{
		onShowMessage: func(params semanticapi.ShowMessageParams) {
			if strings.Contains(params.Message, "Finished loading packages") {
				ready.Do(func() { close(readyCh) })
			}
		},
		onProgress: readyOnProgress(&ready, readyCh),
	}

	mgr := idelsp.New(
		rootURI, scheme, scheme, &stubPkgManager{bin: goplsBin},
		nil, nil, idelsp.Config{Callback: callback, MaxRetries: 1},
	)
	ctx := context.Background()
	t.Cleanup(func() { _ = mgr.Close() })

	// Initialize the go server explicitly to get capabilities.
	initOpts, err := json.Marshal(map[string]any{
		"langID":  "go",
		"command": goplsBin + " serve",
	})
	require.NoError(t, err)

	capabilities, err := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"signatureHelp": map[string]any{},
			"completion":    map[string]any{},
			"hover":         map[string]any{},
		},
		"window": map[string]any{
			"workDoneProgress": true,
		},
	})
	require.NoError(t, err)

	initResult, err := mgr.Initialize(ctx, semanticapi.InitializeParams{
		RootURI:           "file://" + tmpDir,
		Capabilities:      json.RawMessage(capabilities),
		InitializeOptions: json.RawMessage(initOpts),
	})
	require.NoError(t, err)

	// Verify gopls advertises trigger characters.
	require.NotNil(t, initResult.Capabilities.SignatureHelpProvider,
		"gopls should advertise signature help provider")
	triggerChars := initResult.Capabilities.SignatureHelpProvider.TriggerCharacters
	assert.Contains(t, triggerChars, "(", "trigger chars should include '('")
	assert.Contains(t, triggerChars, ",", "trigger chars should include ','")

	// Open the test file so gopls knows about it.
	mgr.Handle(ctx, textapi.Event{
		Type:    textapi.EventTypeOpen,
		URI:     mainWSURI,
		Content: string(mainContent),
	})
	waitReady(t, readyCh)

	// Set up the editor mock to capture the subscribed event handler.
	var capturedHandler textapi.EventHandler
	var mu sync.Mutex
	var locationCalls []textapi.LocationList
	editor := &mockEditor{
		editorFn: func(uri workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: uri}, nil
		},
		subscribeEventsFn: func(
			_ []textapi.EventType, h textapi.EventHandler,
		) error {
			capturedHandler = h
			return nil
		},
		setLocationListFn: func(_ textapi.Handler, _ textapi.LocationPriority, _ string, list textapi.LocationList) error {
			mu.Lock()
			locationCalls = append(locationCalls, list)
			mu.Unlock()
			return nil
		},
	}

	wm := &mockWindowManager{}

	cfg := DefaultConfig()
	cfg.RootURI = rootURI
	cfg.ScheduleNextTick = syncTick
	cfg.SignatureHelp.TriggerCharacters = triggerChars

	_, err = AllHandler(mgr, editor, wm, &mockResourceOpener{},
		&mockNotifications{}, &mockFileSystem{
			openFileFn: func(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
				return os.OpenFile(path, flag, mode)
			},
		}, cfg)
	require.NoError(t, err)
	require.NotNil(t, capturedHandler, "SubscribeEvents should have been called")

	// Simulate typing "(" after "Add" on line 45 (0-indexed),
	// where `fmt.Println(Add(1, 2))` is.
	// The "(" after "Add" is at column 16 (0-indexed: col 16).

	// Send the edit event to gopls via mgr.Handle.
	editEv := textapi.Event{
		Type:     textapi.EventTypeEdit,
		URI:      mainWSURI,
		Resource: &mockHandler{uri: mainWSURI},
		Start:    term.Coordinates{X: 16, Y: 45},
		End:      term.Coordinates{X: 16, Y: 45},
		From:     term.Coordinates{X: 16, Y: 45},
		To:       term.Coordinates{X: 17, Y: 45},
		Content:  "(",
	}
	mgr.Handle(ctx, editEv)

	// Give gopls a moment to process the didChange.
	time.Sleep(500 * time.Millisecond)

	// Fire the same event through the captured handler.
	done := capturedHandler.Handle(ctx, editEv)
	assert.False(t, done, "handler should not signal done")

	// Wait for the async fetch to set the location.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, l := range locationCalls {
			if l != nil {
				return true
			}
		}
		return false
	}, 5*time.Second, 50*time.Millisecond, "SetLocationList should be called with a non-nil list")

	// Verify the location message contains the Add signature with bold markers.
	mu.Lock()
	var gotLoc textapi.Location
	for _, l := range locationCalls {
		if l != nil {
			gotLoc, _ = l.Current()
			break
		}
	}
	mu.Unlock()
	assert.Contains(t, gotLoc.Message, "**", "message should contain bold markers")
	assert.Contains(t, gotLoc.Message, "Add(", "message should contain the Add signature")

	// Fire a cursor event and verify the location list is cleared.
	capturedHandler.Handle(ctx, textapi.Event{
		Type: textapi.EventTypeCursor,
		URI:  mainWSURI,
		From: term.Coordinates{X: 18, Y: 45},
	})

	mu.Lock()
	lastCall := locationCalls[len(locationCalls)-1]
	mu.Unlock()
	assert.Nil(t, lastCall, "cursor event should clear the location list")
}
