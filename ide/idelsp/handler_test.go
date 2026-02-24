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

package idelsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

func TestCallbackHandler_ShowMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		msgType       semanticapi.MessageType
		message       string
		expectedLevel browserapi.NotificationLevel
	}{
		{
			name:          "error message",
			msgType:       semanticapi.MessageTypeError,
			message:       "something failed",
			expectedLevel: browserapi.LevelError,
		},
		{
			name:          "warning message",
			msgType:       semanticapi.MessageTypeWarning,
			message:       "might be wrong",
			expectedLevel: browserapi.LevelWarn,
		},
		{
			name:          "info message",
			msgType:       semanticapi.MessageTypeInfo,
			message:       "all good",
			expectedLevel: browserapi.LevelInfo,
		},
		{
			name:          "log message",
			msgType:       semanticapi.MessageTypeLog,
			message:       "debug info",
			expectedLevel: browserapi.LevelInfo,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			notif := &mockNotifications{}
			h := NewCallbackHandler(
				notif, nil, nil, nil, nil,
				"",
				CallbackHandlerConfig{},
			)
			ctx := ContextWithMetadata(
				t.Context(),
				Metadata{ServerName: "test-server"},
			)
			err := h.ShowMessage(ctx,
				semanticapi.ShowMessageParams{
					Type:    tt.msgType,
					Message: tt.message,
				},
			)
			require.NoError(t, err)
			require.Len(t, notif.notified, 1)
			assert.Equal(t,
				tt.expectedLevel, notif.notified[0].level,
			)
			assert.Equal(t,
				"test-server: "+tt.message,
				notif.notified[0].msg,
			)
		})
	}

	t.Run("no metadata omits prefix", func(t *testing.T) {
		t.Parallel()
		notif := &mockNotifications{}
		h := NewCallbackHandler(
			notif, nil, nil, nil, nil,
			"",
			CallbackHandlerConfig{},
		)
		err := h.ShowMessage(t.Context(),
			semanticapi.ShowMessageParams{
				Type:    semanticapi.MessageTypeInfo,
				Message: "bare message",
			},
		)
		require.NoError(t, err)
		require.Len(t, notif.notified, 1)
		assert.Equal(t,
			"bare message", notif.notified[0].msg,
		)
	})
}

func TestCallbackHandler_LogMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		msgType semanticapi.MessageType
		message string
	}{
		{
			name:    "error",
			msgType: semanticapi.MessageTypeError,
			message: "err msg",
		},
		{
			name:    "warning",
			msgType: semanticapi.MessageTypeWarning,
			message: "warn msg",
		},
		{
			name:    "info",
			msgType: semanticapi.MessageTypeInfo,
			message: "info msg",
		},
		{
			name:    "debug",
			msgType: semanticapi.MessageTypeDebug,
			message: "debug msg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := NewCallbackHandler(
				nil, nil, nil, nil, nil,
				"",
				CallbackHandlerConfig{},
			)
			err := h.LogMessage(t.Context(),
				semanticapi.LogMessageParams{
					Type:    tt.msgType,
					Message: tt.message,
				},
			)
			require.NoError(t, err)
		})
	}
}

func TestCallbackHandler_PublishDiagnostics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		diagnostics       []semanticapi.Diagnostic
		expectedLocations []textapi.Location
		expectedPriority  textapi.LocationPriority
	}{
		{
			name:             "empty clears list",
			diagnostics:      nil,
			expectedPriority: textapi.LocationPriorityInfo,
		},
		{
			name: "single error",
			diagnostics: []semanticapi.Diagnostic{
				{
					Range: semanticapi.Range{
						Start: semanticapi.Position{
							Line: 5, Character: 0,
						},
						End: semanticapi.Position{
							Line: 5, Character: 10,
						},
					},
					Severity: semanticapi.DiagnosticSeverityError,
					Message:  "undefined variable",
				},
			},
			expectedLocations: []textapi.Location{
				{
					From:    term.Coordinates{X: 0, Y: 5},
					To:      term.Coordinates{X: 10, Y: 5},
					Message: "undefined variable",
					Attr: term.Attributes(tcell.Style{
						Bg: tcell.ColorRed,
					}),
				},
			},
			expectedPriority: textapi.LocationPriorityError,
		},
		{
			name: "mixed severities uses highest",
			diagnostics: []semanticapi.Diagnostic{
				{
					Range: semanticapi.Range{
						Start: semanticapi.Position{
							Line: 1, Character: 0,
						},
						End: semanticapi.Position{
							Line: 1, Character: 5,
						},
					},
					Severity: semanticapi.DiagnosticSeverityWarning,
					Message:  "unused var",
				},
				{
					Range: semanticapi.Range{
						Start: semanticapi.Position{
							Line: 3, Character: 0,
						},
						End: semanticapi.Position{
							Line: 3, Character: 8,
						},
					},
					Severity: semanticapi.DiagnosticSeverityError,
					Message:  "syntax error",
				},
			},
			expectedLocations: []textapi.Location{
				{
					From:    term.Coordinates{X: 0, Y: 1},
					To:      term.Coordinates{X: 5, Y: 1},
					Message: "unused var",
					Attr: term.Attributes(tcell.Style{
						Bg: tcell.ColorYellow,
					}),
				},
				{
					From:    term.Coordinates{X: 0, Y: 3},
					To:      term.Coordinates{X: 8, Y: 3},
					Message: "syntax error",
					Attr: term.Attributes(tcell.Style{
						Bg: tcell.ColorRed,
					}),
				},
			},
			expectedPriority: textapi.LocationPriorityError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uri, err := workspaceapi.ParseURI("file:///tmp/test.go")
			require.NoError(t, err)
			ed := &mockEditor{
				handler: &mockEditorHandler{uri: uri},
			}
			h := NewCallbackHandler(
				nil, nil, nil, ed, nil,
				"",
				CallbackHandlerConfig{},
			)
			err = h.PublishDiagnostics(t.Context(),
				semanticapi.PublishDiagnosticsParams{
					URI:         "file:///tmp/test.go",
					Diagnostics: tt.diagnostics,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, "lsp-diagnostics", ed.locID)
			assert.Equal(t, tt.expectedPriority, ed.locPriority)
			assert.Equal(t, tt.expectedLocations, ed.locations)
		})
	}
}

func TestCallbackHandler_Progress(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func(*CallbackHandler)
		token       semanticapi.ProgressToken
		value       string
		checkNotif  func(*testing.T, *mockNotifications)
		checkUpdate func(*testing.T, *mockNotifications)
	}{
		{
			name: "begin creates notification",
			token: semanticapi.ProgressToken{
				StringValue: "tok1",
			},
			value: `{"kind":"begin","title":"Loading"}`,
			checkNotif: func(
				t *testing.T, n *mockNotifications,
			) {
				require.Len(t, n.notified, 1)
				assert.Equal(t,
					browserapi.LevelInfo,
					n.notified[0].level,
				)
				assert.Equal(t,
					"Loading", n.notified[0].msg,
				)
			},
		},
		{
			name: "begin with message appends",
			token: semanticapi.ProgressToken{
				StringValue: "tok2",
			},
			value: `{"kind":"begin","title":"Build","message":"starting"}`,
			checkNotif: func(
				t *testing.T, n *mockNotifications,
			) {
				require.Len(t, n.notified, 1)
				assert.Equal(t,
					"Build: starting",
					n.notified[0].msg,
				)
			},
		},
		{
			name: "report updates progress",
			setup: func(h *CallbackHandler) {
				h.mu.Lock()
				h.progress["tok3"] = "notif-1"
				h.mu.Unlock()
			},
			token: semanticapi.ProgressToken{
				StringValue: "tok3",
			},
			value: `{"kind":"report","message":"50%","percentage":50}`,
			checkUpdate: func(
				t *testing.T, n *mockNotifications,
			) {
				require.Len(t, n.progUpds, 1)
				assert.Equal(t,
					"notif-1", n.progUpds[0].id,
				)
				assert.Equal(t,
					int64(50), n.progUpds[0].progress,
				)
			},
		},
		{
			name: "end removes and completes",
			setup: func(h *CallbackHandler) {
				h.mu.Lock()
				h.progress["tok4"] = "notif-2"
				h.mu.Unlock()
			},
			token: semanticapi.ProgressToken{
				StringValue: "tok4",
			},
			value: `{"kind":"end"}`,
			checkUpdate: func(
				t *testing.T, n *mockNotifications,
			) {
				require.Len(t, n.progUpds, 1)
				assert.Equal(t,
					int64(100), n.progUpds[0].progress,
				)
			},
		},
		{
			name: "integer token",
			token: semanticapi.ProgressToken{
				IntegerValue: 42,
				IsInteger:    true,
			},
			value: `{"kind":"begin","title":"Indexing"}`,
			checkNotif: func(
				t *testing.T, n *mockNotifications,
			) {
				require.Len(t, n.notified, 1)
				assert.Equal(t,
					"Indexing", n.notified[0].msg,
				)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			notif := &mockNotifications{}
			h := NewCallbackHandler(
				notif, nil, nil, nil, nil,
				"",
				CallbackHandlerConfig{},
			)
			if tt.setup != nil {
				tt.setup(h)
			}
			err := h.Progress(t.Context(),
				semanticapi.ProgressParams{
					Token: tt.token,
					Value: json.RawMessage(tt.value),
				},
			)
			require.NoError(t, err)
			if tt.checkNotif != nil {
				tt.checkNotif(t, notif)
			}
			if tt.checkUpdate != nil {
				tt.checkUpdate(t, notif)
			}
		})
	}
}

func TestCallbackHandler_LogTrace(t *testing.T) {
	t.Parallel()
	h := NewCallbackHandler(
		nil, nil, nil, nil, nil,
		"",
		CallbackHandlerConfig{},
	)
	err := h.LogTrace(t.Context(),
		semanticapi.LogTraceParams{
			Message: "trace msg",
			Verbose: "verbose detail",
		},
	)
	require.NoError(t, err)
}

func TestCallbackHandler_ShowDocument(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		params     semanticapi.ShowDocumentParams
		hasEditor  bool
		wantCursor *term.Coordinates
	}{
		{
			name: "opens resource and notifies",
			params: semanticapi.ShowDocumentParams{
				URI: "file:///tmp/foo.go",
			},
		},
		{
			name: "sets cursor on selection",
			params: semanticapi.ShowDocumentParams{
				URI: "file:///tmp/foo.go",
				Selection: &semanticapi.Range{
					Start: semanticapi.Position{
						Line: 10, Character: 5,
					},
					End: semanticapi.Position{
						Line: 10, Character: 15,
					},
				},
			},
			hasEditor: true,
			wantCursor: &term.Coordinates{
				X: 5, Y: 10,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			notif := &mockNotifications{}
			opener := &mockResourceOpener{}
			uri, err := workspaceapi.ParseURI(tt.params.URI)
			require.NoError(t, err)
			var ed Editor
			if tt.hasEditor {
				ed = &mockEditor{
					handler: &mockEditorHandler{uri: uri},
				}
			}
			h := NewCallbackHandler(
				notif, nil, opener, ed, nil,
				"",
				CallbackHandlerConfig{},
			)
			ctx := ContextWithMetadata(
				t.Context(),
				Metadata{ServerName: "test-server"},
			)
			result, err := h.ShowDocument(ctx, tt.params)
			require.NoError(t, err)
			assert.True(t, result.Success)
			require.Len(t, notif.notified, 1)
			assert.Contains(t,
				notif.notified[0].msg, "test-server: see",
			)
			if tt.wantCursor != nil {
				me := ed.(*mockEditor)
				require.NotNil(t, me.cursorSet)
				assert.Equal(t, *tt.wantCursor, *me.cursorSet)
			}
		})
	}
}

func TestCallbackHandler_WorkDoneProgressCreate(
	t *testing.T,
) {
	t.Parallel()
	tests := []struct {
		name  string
		token semanticapi.ProgressToken
		key   string
	}{
		{
			name: "string token",
			token: semanticapi.ProgressToken{
				StringValue: "my-token",
			},
			key: "my-token",
		},
		{
			name: "integer token",
			token: semanticapi.ProgressToken{
				IntegerValue: 99,
				IsInteger:    true,
			},
			key: "99",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := NewCallbackHandler(
				nil, nil, nil, nil, nil,
				"",
				CallbackHandlerConfig{},
			)
			err := h.WorkDoneProgressCreate(
				t.Context(),
				semanticapi.WorkDoneProgressCreateParams{
					Token: tt.token,
				},
			)
			require.NoError(t, err)
			h.mu.Lock()
			_, ok := h.progress[tt.key]
			h.mu.Unlock()
			assert.True(t, ok)
		})
	}
}

func TestCallbackHandler_ApplyEdit(t *testing.T) {
	t.Parallel()
	uri, err := workspaceapi.ParseURI("file:///tmp/test.go")
	require.NoError(t, err)

	tests := []struct {
		name          string
		params        semanticapi.ApplyWorkspaceEditParams
		applied       bool
		expectedEdits []mockCellEdit
	}{
		{
			name: "empty edit succeeds",
			params: semanticapi.ApplyWorkspaceEditParams{
				Edit: semanticapi.WorkspaceEdit{},
			},
			applied: true,
		},
		{
			name: "changes applies text edits",
			params: semanticapi.ApplyWorkspaceEditParams{
				Edit: semanticapi.WorkspaceEdit{
					Changes: map[string][]semanticapi.TextEdit{
						"file:///tmp/test.go": {
							{
								Range: semanticapi.Range{
									Start: semanticapi.Position{
										Line: 0, Character: 0,
									},
									End: semanticapi.Position{
										Line: 0, Character: 3,
									},
								},
								NewText: "hello",
							},
						},
					},
				},
			},
			applied: true,
			expectedEdits: []mockCellEdit{
				{
					start: term.Coordinates{X: 0, Y: 0},
					end:   term.Coordinates{X: 3, Y: 0},
					text:  "hello",
				},
			},
		},
		{
			name: "document changes with text edit",
			params: semanticapi.ApplyWorkspaceEditParams{
				Edit: semanticapi.WorkspaceEdit{
					DocumentChanges: []semanticapi.DocumentChange{
						{
							TextDocumentEdit: &semanticapi.TextDocumentEdit{
								TextDocument: semanticapi.VersionedTextDocumentIdentifier{
									URI: "file:///tmp/test.go",
								},
								Edits: []semanticapi.TextEdit{
									{
										Range: semanticapi.Range{
											Start: semanticapi.Position{
												Line: 1, Character: 0,
											},
											End: semanticapi.Position{
												Line: 1, Character: 5,
											},
										},
										NewText: "world",
									},
								},
							},
						},
					},
				},
			},
			applied: true,
			expectedEdits: []mockCellEdit{
				{
					start: term.Coordinates{X: 0, Y: 1},
					end:   term.Coordinates{X: 5, Y: 1},
					text:  "world",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ed := &mockEditor{
				handler: &mockEditorHandler{uri: uri},
			}
			h := NewCallbackHandler(
				nil, nil, nil, ed,
				newTestScheme(),
				"",
				CallbackHandlerConfig{},
			)
			result, err := h.ApplyEdit(t.Context(), tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.applied, result.Applied)
			assert.Equal(t, tt.expectedEdits, ed.cellEdits)
		})
	}
}

func TestCallbackHandler_ApplyEdit_CreateFile(
	t *testing.T,
) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })
	newFile := filepath.Join(tmpDir, "new.go")

	h := NewCallbackHandler(
		nil, nil, nil, nil, newTestScheme(),
		"",
		CallbackHandlerConfig{},
	)
	result, err := h.ApplyEdit(t.Context(),
		semanticapi.ApplyWorkspaceEditParams{
			Edit: semanticapi.WorkspaceEdit{
				DocumentChanges: []semanticapi.DocumentChange{
					{
						CreateFile: &semanticapi.CreateFile{
							Kind: "create",
							URI:  "file://" + newFile,
						},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	assert.True(t, result.Applied)
	_, err = os.Stat(newFile)
	require.NoError(t, err)
}

func TestCallbackHandler_ApplyEdit_DeleteFile(
	t *testing.T,
) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })
	target := filepath.Join(tmpDir, "del.go")
	require.NoError(t, os.WriteFile(
		target, []byte("x"), 0644,
	))

	h := NewCallbackHandler(
		nil, nil, nil, nil, newTestScheme(),
		"",
		CallbackHandlerConfig{},
	)
	result, err := h.ApplyEdit(t.Context(),
		semanticapi.ApplyWorkspaceEditParams{
			Edit: semanticapi.WorkspaceEdit{
				DocumentChanges: []semanticapi.DocumentChange{
					{
						DeleteFile: &semanticapi.DeleteFile{
							Kind: "delete",
							URI:  "file://" + target,
						},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	assert.True(t, result.Applied)
	_, err = os.Stat(target)
	require.True(t, os.IsNotExist(err))
}

func TestCallbackHandler_ApplyEdit_RenameFile(
	t *testing.T,
) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })
	oldFile := filepath.Join(tmpDir, "old.go")
	newFile := filepath.Join(tmpDir, "new.go")
	require.NoError(t, os.WriteFile(
		oldFile, []byte("content"), 0644,
	))

	h := NewCallbackHandler(
		nil, nil, nil, nil, newTestScheme(),
		"",
		CallbackHandlerConfig{},
	)
	result, err := h.ApplyEdit(t.Context(),
		semanticapi.ApplyWorkspaceEditParams{
			Edit: semanticapi.WorkspaceEdit{
				DocumentChanges: []semanticapi.DocumentChange{
					{
						RenameFile: &semanticapi.RenameFile{
							Kind:   "rename",
							OldURI: "file://" + oldFile,
							NewURI: "file://" + newFile,
						},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	assert.True(t, result.Applied)
	_, err = os.Stat(oldFile)
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(newFile)
	require.NoError(t, err)
}

func TestCallbackHandler_ApplyEdit_IgnoreIfExists(
	t *testing.T,
) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })
	existing := filepath.Join(tmpDir, "exists.go")
	require.NoError(t, os.WriteFile(
		existing, []byte("original"), 0644,
	))

	h := NewCallbackHandler(
		nil, nil, nil, nil, newTestScheme(),
		"",
		CallbackHandlerConfig{},
	)
	result, err := h.ApplyEdit(t.Context(),
		semanticapi.ApplyWorkspaceEditParams{
			Edit: semanticapi.WorkspaceEdit{
				DocumentChanges: []semanticapi.DocumentChange{
					{
						CreateFile: &semanticapi.CreateFile{
							Kind: "create",
							URI:  "file://" + existing,
							Options: &semanticapi.CreateFileOptions{
								IgnoreIfExists: true,
							},
						},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	assert.True(t, result.Applied)
	data, err := os.ReadFile(existing)
	require.NoError(t, err)
	assert.Equal(t, "original", string(data))
}

func TestCallbackHandler_ApplyEdit_DeleteIgnoreIfNotExists(
	t *testing.T,
) {
	t.Parallel()
	h := NewCallbackHandler(
		nil, nil, nil, nil, newTestScheme(),
		"",
		CallbackHandlerConfig{},
	)
	result, err := h.ApplyEdit(t.Context(),
		semanticapi.ApplyWorkspaceEditParams{
			Edit: semanticapi.WorkspaceEdit{
				DocumentChanges: []semanticapi.DocumentChange{
					{
						DeleteFile: &semanticapi.DeleteFile{
							Kind: "delete",
							URI:  "file:///nonexistent/path.go",
							Options: &semanticapi.DeleteFileOptions{
								IgnoreIfNotExists: true,
							},
						},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	assert.True(t, result.Applied)
}

func TestCallbackHandler_WorkspaceFolders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		rootURI string
		want    []semanticapi.WorkspaceFolder
	}{
		{
			name:    "standard path",
			rootURI: "file:///home/user/project",
			want: []semanticapi.WorkspaceFolder{
				{
					URI:  "file:///home/user/project",
					Name: "project",
				},
			},
		},
		{
			name:    "nested path",
			rootURI: "file:///a/b/c",
			want: []semanticapi.WorkspaceFolder{
				{
					URI:  "file:///a/b/c",
					Name: "c",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := NewCallbackHandler(
				nil, nil, nil, nil, nil,
				tt.rootURI,
				CallbackHandlerConfig{},
			)
			folders, err := h.WorkspaceFolders(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tt.want, folders)
		})
	}
}

func TestCallbackHandler_Configuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		config   config.Config
		items    []semanticapi.ConfigurationItem
		expected []string
	}{
		{
			name:   "nil config returns null",
			config: nil,
			items: []semanticapi.ConfigurationItem{
				{Section: "gopls"},
			},
			expected: []string{"null"},
		},
		{
			name: "returns map as JSON",
			config: config.MapConfig(map[string]any{
				"lsp": map[string]any{
					"servers": map[string]any{
						"gopls": map[string]any{
							"usePlaceholders": true,
						},
					},
				},
			}),
			items: []semanticapi.ConfigurationItem{
				{Section: "gopls"},
			},
			expected: []string{
				`{"usePlaceholders":true}`,
			},
		},
		{
			name: "missing key returns null",
			config: config.MapConfig(map[string]any{
				"lsp": map[string]any{},
			}),
			items: []semanticapi.ConfigurationItem{
				{Section: "missing"},
			},
			expected: []string{"null"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := NewCallbackHandler(
				nil, nil, nil, nil, nil,
				"",
				CallbackHandlerConfig{
					Config: tt.config,
				},
			)
			results, err := h.Configuration(
				t.Context(),
				semanticapi.ConfigurationParams{
					Items: tt.items,
				},
			)
			require.NoError(t, err)
			require.Len(t, results, len(tt.expected))
			for i, exp := range tt.expected {
				assert.JSONEq(t,
					exp, string(results[i]),
				)
			}
		})
	}
}

func TestCallbackHandler_RegisterUnregister(
	t *testing.T,
) {
	t.Parallel()
	h := NewCallbackHandler(
		nil, nil, nil, nil, nil,
		"",
		CallbackHandlerConfig{},
	)
	assert.NoError(t, h.RegisterCapability(
		t.Context(), semanticapi.RegistrationParams{},
	))
	assert.NoError(t, h.UnregisterCapability(
		t.Context(), semanticapi.UnregistrationParams{},
	))
}

func TestCallbackHandler_Refresh(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		call  func(context.Context, *CallbackHandler) error
		check func(*testing.T, *mockRefresher)
	}{
		{
			name: "CodeLensRefresh",
			call: func(
				ctx context.Context, h *CallbackHandler,
			) error {
				return h.CodeLensRefresh(ctx)
			},
			check: func(t *testing.T, r *mockRefresher) {
				assert.Equal(t, 1, r.codeLens)
			},
		},
		{
			name: "SemanticTokensRefresh",
			call: func(
				ctx context.Context, h *CallbackHandler,
			) error {
				return h.SemanticTokensRefresh(ctx)
			},
			check: func(t *testing.T, r *mockRefresher) {
				assert.Equal(t, 1, r.semantic)
			},
		},
		{
			name: "InlayHintRefresh",
			call: func(
				ctx context.Context, h *CallbackHandler,
			) error {
				return h.InlayHintRefresh(ctx)
			},
			check: func(t *testing.T, r *mockRefresher) {
				assert.Equal(t, 1, r.inlayHints)
			},
		},
		{
			name: "DiagnosticRefresh",
			call: func(
				ctx context.Context, h *CallbackHandler,
			) error {
				return h.DiagnosticRefresh(ctx)
			},
			check: func(t *testing.T, r *mockRefresher) {
				assert.Equal(t, 1, r.diagnostics)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &mockRefresher{}
			h := NewCallbackHandler(
				nil, nil, nil, nil, nil,
				"",
				CallbackHandlerConfig{Refresher: r},
			)
			err := tt.call(t.Context(), h)
			require.NoError(t, err)
			tt.check(t, r)
		})
	}
}

func TestCallbackHandler_NopRefresherDefault(
	t *testing.T,
) {
	t.Parallel()
	h := NewCallbackHandler(
		nil, nil, nil, nil, nil,
		"",
		CallbackHandlerConfig{},
	)
	assert.NoError(t, h.CodeLensRefresh(t.Context()))
	assert.NoError(t,
		h.SemanticTokensRefresh(t.Context()),
	)
	assert.NoError(t, h.InlayHintRefresh(t.Context()))
	assert.NoError(t, h.DiagnosticRefresh(t.Context()))
}

func TestProgressTokenKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		token semanticapi.ProgressToken
		want  string
	}{
		{
			name: "string token",
			token: semanticapi.ProgressToken{
				StringValue: "abc",
			},
			want: "abc",
		},
		{
			name: "integer token",
			token: semanticapi.ProgressToken{
				IntegerValue: 42,
				IsInteger:    true,
			},
			want: "42",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t,
				tt.want, progressTokenKey(tt.token),
			)
		})
	}
}

func TestDiagnosticSeverityToAttr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		severity semanticapi.DiagnosticSeverity
		wantBg   tcell.Color
	}{
		{
			name:     "error is red",
			severity: semanticapi.DiagnosticSeverityError,
			wantBg:   tcell.ColorRed,
		},
		{
			name:     "warning is yellow",
			severity: semanticapi.DiagnosticSeverityWarning,
			wantBg:   tcell.ColorYellow,
		},
		{
			name:     "info is blue",
			severity: semanticapi.DiagnosticSeverityInformation,
			wantBg:   tcell.ColorBlue,
		},
		{
			name:     "hint is gray",
			severity: semanticapi.DiagnosticSeverityHint,
			wantBg:   tcell.ColorGray,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			attr := diagnosticSeverityToAttr(tt.severity)
			style := tcell.Style(attr)
			assert.Equal(t, tt.wantBg, style.Bg)
		})
	}
}

type mockNotifications struct {
	mu        sync.Mutex
	notified  []mockNotification
	progUpds  []mockProgressUpdate
	nextID    int
	notifyErr error
}

type mockNotification struct {
	level browserapi.NotificationLevel
	msg   string
}

type mockProgressUpdate struct {
	id       string
	message  string
	progress int64
	total    int64
}

func (m *mockNotifications) Notify(
	level browserapi.NotificationLevel,
	msg string, args ...any,
) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.notifyErr != nil {
		return "", m.notifyErr
	}
	m.nextID++
	id := "notif-" + string(rune('0'+m.nextID))
	formatted := msg
	if len(args) > 0 {
		formatted = fmt.Sprintf(msg, args...)
	}
	m.notified = append(m.notified, mockNotification{
		level: level, msg: formatted,
	})
	return id, nil
}

func (m *mockNotifications) NotifyOnce(
	level browserapi.NotificationLevel,
	msg string, args ...any,
) (string, error) {
	return m.Notify(level, msg, args...)
}

func (m *mockNotifications) UpdateNotificationProgress(
	id, message string, progress, total int64,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progUpds = append(m.progUpds, mockProgressUpdate{
		id: id, message: message,
		progress: progress, total: total,
	})
	return nil
}

type mockEditor struct {
	mu          sync.Mutex
	handler     *mockEditorHandler
	locations   []textapi.Location
	locPriority textapi.LocationPriority
	locID       string
	cursorSet   *term.Coordinates
	cellEdits   []mockCellEdit
	editorErr   error
	locationErr error
}

func (m *mockEditor) Editor(
	_ workspaceapi.URI,
) (textapi.Handler, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.editorErr != nil {
		return nil, m.editorErr
	}
	return m.handler, nil
}

func (m *mockEditor) SetLocationList(
	_ textapi.Handler, priority textapi.LocationPriority,
	id string, list textapi.LocationList,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.locationErr != nil {
		return m.locationErr
	}
	m.locPriority = priority
	m.locID = id
	m.locations = nil
	if list != nil {
		for loc, ok := list.Current(); ok; loc, ok = list.Next() {
			m.locations = append(m.locations, loc)
		}
	}
	return nil
}

func (m *mockEditor) SetCursor(
	_ textapi.Handler, c term.Coordinates,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cursorSet = &c
	return nil
}

func (m *mockEditor) CellEditor(
	_ textapi.Handler,
) textapi.CellEditor {
	return &mockCellEditor{editor: m}
}

type mockEditorHandler struct {
	uri workspaceapi.URI
}

func (m *mockEditorHandler) Resource() workspaceapi.URI {
	return m.uri
}

func (m *mockEditorHandler) Resize(_, _ int)    {}
func (m *mockEditorHandler) Draw(_ term.Writer) {}
func (m *mockEditorHandler) Handle(
	_ term.Event,
) (bool, bool) {
	return false, false
}
func (m *mockEditorHandler) Cursor() (
	term.Coordinates, term.CursorStyle, bool,
) {
	return term.Coordinates{}, 0, false
}
func (m *mockEditorHandler) Selection() (string, bool) {
	return "", false
}
func (m *mockEditorHandler) Close() error {
	return nil
}

type mockCellEdit struct {
	start, end term.Coordinates
	text       string
}

type mockCellEditor struct {
	editor *mockEditor
}

func (c *mockCellEditor) Edit(
	_ context.Context,
	start, end term.Coordinates, str string,
) (term.Coordinates, term.Coordinates, string, error) {
	c.editor.mu.Lock()
	defer c.editor.mu.Unlock()
	c.editor.cellEdits = append(
		c.editor.cellEdits,
		mockCellEdit{start: start, end: end, text: str},
	)
	return start, end, "", nil
}

type mockResourceOpener struct {
	mu     sync.Mutex
	opened []workspaceapi.URI
}

func (m *mockResourceOpener) Open(
	uri workspaceapi.URI,
) (browserapi.Handler, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.opened = append(m.opened, uri)
	return nil, nil
}

type mockRefresher struct {
	mu          sync.Mutex
	codeLens    int
	semantic    int
	inlayHints  int
	diagnostics int
}

func (m *mockRefresher) RefreshCodeLens(
	_ context.Context,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codeLens++
	return nil
}

func (m *mockRefresher) RefreshSemanticTokens(
	_ context.Context,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.semantic++
	return nil
}

func (m *mockRefresher) RefreshInlayHints(
	_ context.Context,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inlayHints++
	return nil
}

func (m *mockRefresher) RefreshDiagnostics(
	_ context.Context,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.diagnostics++
	return nil
}
