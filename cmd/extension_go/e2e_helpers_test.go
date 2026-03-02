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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/blue/ide/idelsp"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// findGopls locates the gopls binary or skips the test.
func findGopls(t *testing.T) string {
	t.Helper()
	goplsBin, err := exec.LookPath("gopls")
	if err != nil {
		for _, p := range []string{
			filepath.Join(os.Getenv("HOME"), ".rune", "bin", "gopls"),
			filepath.Join(os.Getenv("HOME"), "go", "bin", "gopls"),
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		t.Skip("gopls not found, skipping e2e test")
	}
	return goplsBin
}

// testFile describes a file to create in the test workspace.
type testFile struct {
	name    string
	content string
}

// setupWorkspace creates a temp directory with a go.mod and the given files.
// It returns the temp directory path.
func setupWorkspace(t *testing.T, moduleName string, files []testFile) string {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "go-ext-test-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	goMod := fmt.Sprintf("module %s\n\ngo 1.22\n", moduleName)
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644,
	))

	for _, f := range files {
		dir := filepath.Dir(filepath.Join(tmpDir, f.name))
		if dir != tmpDir {
			require.NoError(t, os.MkdirAll(dir, 0o755))
		}
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, f.name), []byte(f.content), 0o644,
		))
	}

	return tmpDir
}

// testEnv holds the initialized gopls manager and file URIs for a test.
type testEnv struct {
	mgr      *idelsp.Manager
	cb       *testCallback
	dir      string
	fileURIs map[string]string // filename -> file:// URI
}

// initGopls creates a new idelsp.Manager, initializes gopls with the
// extension's goplsInitializeParams, opens the specified files via
// Handle(EventTypeOpen), and waits for gopls to finish loading.
func initGopls(t *testing.T, goplsBin string, files []testFile) *testEnv {
	t.Helper()

	dir := setupWorkspace(t, "example.com/test", files)
	rootURI := "file://" + dir

	uri, err := workspaceapi.ParseURI(rootURI)
	require.NoError(t, err)

	scheme := newTestScheme()

	ready := make(chan struct{})
	var once sync.Once
	cb := &testCallback{
		onShowMessage: func(params semanticapi.ShowMessageParams) {
			if strings.Contains(params.Message, "Finished loading packages") ||
				strings.Contains(params.Message, "background refresh finished") {
				once.Do(func() { close(ready) })
			}
		},
		onProgress: readyOnProgressCh(&once, ready),
	}
	cfg := idelsp.Config{MaxRetries: 1, Callback: cb}

	mgr := idelsp.New(
		uri,
		scheme,
		scheme,
		&stubPkgManager{bin: goplsBin},
		nil, // notifications
		nil, // opener
		cfg,
	)

	ctx := context.Background()

	params, err := goplsInitializeParams(rootURI)
	require.NoError(t, err)

	_, err = mgr.Initialize(ctx, params)
	require.NoError(t, err)

	fileURIs := make(map[string]string)
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		fileURI := "file://" + path
		fileURIs[f.name] = fileURI

		wsURI, err := workspaceapi.ParseURI(fileURI)
		require.NoError(t, err)
		mgr.Handle(ctx, textapi.Event{
			Type:    textapi.EventTypeOpen,
			URI:     wsURI,
			Content: f.content,
		})
	}

	waitGoplsReady(t, ready, mgr, fileURIs)

	t.Cleanup(func() {
		require.NoError(t, mgr.Close())
	})

	return &testEnv{
		mgr:      mgr,
		cb:       cb,
		dir:      dir,
		fileURIs: fileURIs,
	}
}

// applyEditEnv extends testEnv with captured workspace/applyEdit params.
type applyEditEnv struct {
	*testEnv
	cb *testCallback
}

func (e *applyEditEnv) capturedEdits() []semanticapi.ApplyWorkspaceEditParams {
	e.cb.mu.Lock()
	defer e.cb.mu.Unlock()
	return append([]semanticapi.ApplyWorkspaceEditParams{}, e.cb.appliedEdits...)
}

// simulateEditorEvents sets up onApplyEdit to fire Handle(EventTypeEdit)
// for each edit in workspace/applyEdit, simulating the real IDE where
// buffer changes from ApplyEdit trigger editor events that send
// textDocument/didChange to the language server.
func (e *applyEditEnv) simulateEditorEvents() {
	e.cb.mu.Lock()
	e.cb.onApplyEdit = func(params semanticapi.ApplyWorkspaceEditParams) {
		edit := params.Edit
		if len(edit.DocumentChanges) > 0 {
			for _, dc := range edit.DocumentChanges {
				if dc.TextDocumentEdit == nil {
					continue
				}
				wsURI, err := workspaceapi.ParseURI(dc.TextDocumentEdit.TextDocument.URI)
				if err != nil {
					continue
				}
				for _, te := range dc.TextDocumentEdit.Edits {
					e.mgr.Handle(context.Background(), textapi.Event{
						Type:    textapi.EventTypeEdit,
						URI:     wsURI,
						Content: te.NewText,
						Start:   term.Coordinates{X: int(te.Range.Start.Character), Y: int(te.Range.Start.Line)},
						End:     term.Coordinates{X: int(te.Range.End.Character), Y: int(te.Range.End.Line)},
					})
				}
			}
			return
		}
		for fileURI, edits := range edit.Changes {
			wsURI, err := workspaceapi.ParseURI(fileURI)
			if err != nil {
				continue
			}
			for _, te := range edits {
				e.mgr.Handle(context.Background(), textapi.Event{
					Type:    textapi.EventTypeEdit,
					URI:     wsURI,
					Content: te.NewText,
					Start:   term.Coordinates{X: int(te.Range.Start.Character), Y: int(te.Range.Start.Line)},
					End:     term.Coordinates{X: int(te.Range.End.Character), Y: int(te.Range.End.Line)},
				})
			}
		}
	}
	e.cb.mu.Unlock()
}

// initGoplsWithApplyEdit is like initGopls but captures workspace/applyEdit
// params and simulates editor events (Handle(EventTypeEdit)) so the language
// server's overlay stays in sync — matching real IDE behavior.
func initGoplsWithApplyEdit(
	t *testing.T, goplsBin string, files []testFile,
) *applyEditEnv {
	t.Helper()

	dir := setupWorkspace(t, "example.com/test", files)
	rootURI := "file://" + dir

	uri, err := workspaceapi.ParseURI(rootURI)
	require.NoError(t, err)

	scheme := newTestScheme()

	ready := make(chan struct{})
	var once sync.Once
	cb := &testCallback{
		onShowMessage: func(params semanticapi.ShowMessageParams) {
			if strings.Contains(params.Message, "Finished loading packages") ||
				strings.Contains(params.Message, "background refresh finished") {
				once.Do(func() { close(ready) })
			}
		},
		onProgress: readyOnProgressCh(&once, ready),
	}
	cfg := idelsp.Config{
		MaxRetries: 1,
		Callback:   cb,
	}

	mgr := idelsp.New(
		uri, scheme, scheme, &stubPkgManager{bin: goplsBin},
		nil, nil, cfg,
	)

	ctx := context.Background()

	params, err := goplsInitializeParams(rootURI)
	require.NoError(t, err)

	_, err = mgr.Initialize(ctx, params)
	require.NoError(t, err)

	fileURIs := make(map[string]string)
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		fileURI := "file://" + path
		fileURIs[f.name] = fileURI

		wsURI, err := workspaceapi.ParseURI(fileURI)
		require.NoError(t, err)
		mgr.Handle(ctx, textapi.Event{
			Type:    textapi.EventTypeOpen,
			URI:     wsURI,
			Content: f.content,
		})
	}

	waitGoplsReady(t, ready, mgr, fileURIs)
	t.Cleanup(func() { require.NoError(t, mgr.Close()) })

	env := &applyEditEnv{
		testEnv: &testEnv{mgr: mgr, dir: dir, fileURIs: fileURIs},
		cb:      cb,
	}
	env.simulateEditorEvents()
	return env
}

// autoInitLikeParams builds params that mirror what Manager.autoInitParams
// produces (no gopls-specific initializationOptions, no workspace.configuration,
// no workspace.workspaceEdit.documentChanges), plus the langID/command keys
// that Manager.Initialize requires.
func autoInitLikeParams(rootURI string) (semanticapi.InitializeParams, error) {
	initOptions := map[string]any{
		"langID":  "go",
		"command": "gopls serve",
	}
	initOptionsData, err := json.Marshal(initOptions)
	if err != nil {
		return semanticapi.InitializeParams{}, err
	}

	capabilities := map[string]any{
		"textDocument": map[string]any{
			"implementation": map[string]any{
				"linkSupport": true,
			},
			"completion":     map[string]any{},
			"hover":          map[string]any{},
			"signatureHelp":  map[string]any{},
			"definition":     map[string]any{},
			"references":     map[string]any{},
			"documentSymbol": map[string]any{},
			"formatting":     map[string]any{},
			"rename": map[string]any{
				"prepareSupport": true,
			},
			"codeAction":        map[string]any{},
			"codeLens":          map[string]any{},
			"foldingRange":      map[string]any{"lineFoldingOnly": false},
			"selectionRange":    map[string]any{},
			"documentHighlight": map[string]any{},
			"callHierarchy":     map[string]any{},
			"inlayHint":         map[string]any{},
			"semanticTokens": map[string]any{
				"requests": map[string]any{"full": true, "range": true},
				"tokenTypes": []string{
					"namespace", "type", "class", "enum", "interface", "struct",
					"typeParameter", "parameter", "variable", "property",
					"enumMember", "event", "function", "method", "macro",
					"keyword", "modifier", "comment", "string", "number",
					"regexp", "operator", "decorator", "label",
				},
				"tokenModifiers": []string{
					"declaration", "definition", "readonly", "static",
					"deprecated", "abstract", "async", "modification",
					"documentation", "defaultLibrary",
				},
				"formats": []string{"relative"},
			},
		},
		"workspace": map[string]any{
			"symbol":      map[string]any{},
			"diagnostics": map[string]any{},
		},
		"window": map[string]any{
			"workDoneProgress": true,
		},
	}
	capsData, err := json.Marshal(capabilities)
	if err != nil {
		return semanticapi.InitializeParams{}, err
	}

	return semanticapi.InitializeParams{
		RootURI:           rootURI,
		Capabilities:      json.RawMessage(capsData),
		InitializeOptions: json.RawMessage(initOptionsData),
	}, nil
}

// initGoplsWithAutoInitParams creates a gopls environment using params
// that mirror Manager.autoInitParams (no gopls-specific settings, no
// workspace.configuration, no workspace.workspaceEdit.documentChanges).
func initGoplsWithAutoInitParams(
	t *testing.T, goplsBin string, files []testFile,
) *applyEditEnv {
	t.Helper()

	dir := setupWorkspace(t, "example.com/test", files)
	rootURI := "file://" + dir

	uri, err := workspaceapi.ParseURI(rootURI)
	require.NoError(t, err)

	scheme := newTestScheme()

	ready := make(chan struct{})
	var once sync.Once
	cb := &testCallback{
		onShowMessage: func(params semanticapi.ShowMessageParams) {
			if strings.Contains(params.Message, "Finished loading packages") ||
				strings.Contains(params.Message, "background refresh finished") {
				once.Do(func() { close(ready) })
			}
		},
		onProgress: readyOnProgressCh(&once, ready),
	}
	cfg := idelsp.Config{
		MaxRetries: 1,
		Callback:   cb,
	}

	mgr := idelsp.New(
		uri, scheme, scheme, &stubPkgManager{bin: goplsBin},
		nil, nil, cfg,
	)

	params, err := autoInitLikeParams(rootURI)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = mgr.Initialize(ctx, params)
	require.NoError(t, err)

	fileURIs := make(map[string]string)
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		fileURI := "file://" + path
		fileURIs[f.name] = fileURI

		wsURI, err := workspaceapi.ParseURI(fileURI)
		require.NoError(t, err)
		mgr.Handle(ctx, textapi.Event{
			Type:    textapi.EventTypeOpen,
			URI:     wsURI,
			Content: f.content,
		})
	}

	waitGoplsReady(t, ready, mgr, fileURIs)
	t.Cleanup(func() { require.NoError(t, mgr.Close()) })

	env := &applyEditEnv{
		testEnv: &testEnv{mgr: mgr, dir: dir, fileURIs: fileURIs},
		cb:      cb,
	}
	env.simulateEditorEvents()
	return env
}

// stubPkgManager implements idelsp.PkgManager for tests.
type stubPkgManager struct {
	bin string
}

func (p *stubPkgManager) LibDir(
	_ context.Context, _ string,
) (iterator.Iterator[string], error) {
	return iterator.FromSlice([]string{p.bin}), nil
}

// testCallback implements semanticapi.LSPCallback for e2e tests.
type testCallback struct {
	mu            sync.Mutex
	onShowMessage func(params semanticapi.ShowMessageParams)
	onProgress    func(semanticapi.ProgressParams)
	onApplyEdit   func(semanticapi.ApplyWorkspaceEditParams)
	appliedEdits  []semanticapi.ApplyWorkspaceEditParams
	diagnostics   []semanticapi.PublishDiagnosticsParams
}

func (c *testCallback) ShowMessage(
	_ context.Context, params semanticapi.ShowMessageParams,
) error {
	c.mu.Lock()
	cb := c.onShowMessage
	c.mu.Unlock()
	if cb != nil {
		cb(params)
	}
	return nil
}

func (c *testCallback) LogMessage(
	_ context.Context, _ semanticapi.LogMessageParams,
) error {
	return nil
}

func (c *testCallback) PublishDiagnostics(
	_ context.Context, params semanticapi.PublishDiagnosticsParams,
) error {
	c.mu.Lock()
	c.diagnostics = append(c.diagnostics, params)
	c.mu.Unlock()
	return nil
}

func (c *testCallback) Progress(
	_ context.Context, params semanticapi.ProgressParams,
) error {
	c.mu.Lock()
	cb := c.onProgress
	c.mu.Unlock()
	if cb != nil {
		cb(params)
	}
	return nil
}

func (c *testCallback) LogTrace(
	_ context.Context, _ semanticapi.LogTraceParams,
) error {
	return nil
}

func (c *testCallback) ShowDocument(
	_ context.Context, _ semanticapi.ShowDocumentParams,
) (semanticapi.ShowDocumentResult, error) {
	return semanticapi.ShowDocumentResult{Success: true}, nil
}

func (c *testCallback) ShowMessageRequest(
	_ context.Context, _ semanticapi.ShowMessageRequestParams,
) (*semanticapi.MessageActionItem, error) {
	return nil, nil
}

func (c *testCallback) WorkDoneProgressCreate(
	_ context.Context, _ semanticapi.WorkDoneProgressCreateParams,
) error {
	return nil
}

func (c *testCallback) ApplyEdit(
	_ context.Context,
	params semanticapi.ApplyWorkspaceEditParams,
) (semanticapi.ApplyWorkspaceEditResult, error) {
	c.mu.Lock()
	c.appliedEdits = append(c.appliedEdits, params)
	cb := c.onApplyEdit
	c.mu.Unlock()
	if cb != nil {
		cb(params)
	}
	return semanticapi.ApplyWorkspaceEditResult{Applied: true}, nil
}

func (c *testCallback) WorkspaceFolders(
	_ context.Context,
) ([]semanticapi.WorkspaceFolder, error) {
	return nil, nil
}

func (c *testCallback) Configuration(
	_ context.Context, params semanticapi.ConfigurationParams,
) ([]json.RawMessage, error) {
	// Return empty settings for each requested item so gopls
	// applies its default configuration (including analyzers).
	result := make([]json.RawMessage, len(params.Items))
	for i := range result {
		result[i] = json.RawMessage(`{}`)
	}
	return result, nil
}

func (c *testCallback) RegisterCapability(
	_ context.Context, _ semanticapi.RegistrationParams,
) error {
	return nil
}

func (c *testCallback) UnregisterCapability(
	_ context.Context, _ semanticapi.UnregistrationParams,
) error {
	return nil
}

func (c *testCallback) CodeLensRefresh(_ context.Context) error {
	return nil
}

func (c *testCallback) SemanticTokensRefresh(_ context.Context) error {
	return nil
}

func (c *testCallback) InlayHintRefresh(_ context.Context) error {
	return nil
}

func (c *testCallback) DiagnosticRefresh(_ context.Context) error {
	return nil
}

// localScheme implements schemeapi.FileSystem and schemeapi.Executor
// using the local OS for e2e testing.
type localScheme struct {
	mu      sync.Mutex
	procs   map[workspaceapi.Pid]*os.Process
	nextPid workspaceapi.Pid
}

func newTestScheme() *localScheme {
	return &localScheme{
		procs:   make(map[workspaceapi.Pid]*os.Process),
		nextPid: 1,
	}
}

func (s *localScheme) Create(filename string) (workspaceapi.File, error) {
	return os.Create(filename)
}

func (s *localScheme) Open(filename string) (workspaceapi.File, error) {
	return os.Open(filename)
}

func (s *localScheme) OpenFile(filename string, flag int, perm os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(filename, flag, perm)
}

func (s *localScheme) Stat(filename string) (os.FileInfo, error) {
	return os.Stat(filename)
}

func (s *localScheme) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (s *localScheme) Remove(filename string) error {
	return os.Remove(filename)
}

func (s *localScheme) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (s *localScheme) TempFile(dir, prefix string) (workspaceapi.File, error) {
	return os.CreateTemp(dir, prefix)
}

func (s *localScheme) Lstat(filename string) (os.FileInfo, error) {
	return os.Lstat(filename)
}

func (s *localScheme) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (s *localScheme) Readlink(link string) (string, error) {
	return os.Readlink(link)
}

func (s *localScheme) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func (s *localScheme) MkdirAll(filename string, perm os.FileMode) error {
	return os.MkdirAll(filename, perm)
}

func (s *localScheme) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	c := exec.CommandContext(ctx, cmd.Path, cmd.Args...)
	if cmd.Dir != "" {
		c.Dir = cmd.Dir
	}
	if cmd.Env != nil {
		c.Env = cmd.Env
	}
	c.Stdin = cmd.Stdin
	c.Stdout = cmd.Stdout
	c.Stderr = cmd.Stderr
	if cmd.SysProcAttr != nil {
		c.SysProcAttr = cmd.SysProcAttr
	}

	if err := c.Start(); err != nil {
		return 0, err
	}

	s.mu.Lock()
	pid := s.nextPid
	s.nextPid++
	s.procs[pid] = c.Process
	s.mu.Unlock()

	if cmd.Watcher != nil {
		ch := cmd.Watcher.WatchProcess()
		go func() {
			err := c.Wait()
			if ch != nil {
				ch <- err
			}
		}()
	}

	return pid, nil
}

func (s *localScheme) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	s.mu.Lock()
	proc, ok := s.procs[pid]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("process %d not found", pid)
	}
	return proc.Signal(sig)
}

func (s *localScheme) Close() error {
	return nil
}

// readyOnProgressCh returns an onProgress callback that closes
// the ready channel (via sync.Once) when a progress sequence ends.
func readyOnProgressCh(
	once *sync.Once, ready chan struct{},
) func(semanticapi.ProgressParams) {
	return func(p semanticapi.ProgressParams) {
		var v struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal(p.Value, &v) != nil {
			return
		}
		if v.Kind == "end" {
			once.Do(func() { close(ready) })
		}
	}
}

// waitGoplsReady waits for gopls to signal that loading is complete.
// It first waits on the ready channel (fed by progress/showMessage
// callbacks). If the signal doesn't arrive within a short window,
// it falls back to polling with Hover to handle simple workspaces
// where gopls may not send progress events.
func waitGoplsReady(
	t *testing.T,
	ready <-chan struct{},
	mgr *idelsp.Manager,
	fileURIs map[string]string,
) {
	t.Helper()

	// Fast path: wait for the progress signal.
	select {
	case <-ready:
		return
	case <-time.After(5 * time.Second):
	}

	// Fallback: poll with Hover until gopls responds.
	var fileURI string
	for _, u := range fileURIs {
		fileURI = u
		break
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	for {
		_, err := mgr.Hover(ctx, semanticapi.HoverParams{
			TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
			Position:     semanticapi.Position{Line: 0, Character: 8},
		})
		if err == nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("gopls not ready: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// ---------------------------------------------------------------------------
// Mock types for handler-level e2e tests
// ---------------------------------------------------------------------------

// parseTestURI parses a file:// URI string into a workspaceapi.URI.
func parseTestURI(t *testing.T, fileURI string) workspaceapi.URI {
	t.Helper()
	u, err := workspaceapi.ParseURI(fileURI)
	require.NoError(t, err)
	return u
}

// newTestHandler creates a go handler wired to the given testEnv's LSP
// manager with mock editor and notifications.
func newTestHandler(
	t *testing.T, env *testEnv,
) (textapi.CommandHandler, *mockEditor, *mockNotifications) {
	t.Helper()
	me := newMockEditor()
	mn := &mockNotifications{}
	_, handler := newGoHandler(env.mgr, me, nil, mn)
	return handler, me, mn
}

// goCmd creates a textapi.Command for the "go" command with the given
// subcommand, URI, and resource.
func goCmd(
	subcommand string, uri workspaceapi.URI, resource textapi.Handler,
) textapi.Command {
	return textapi.Command{
		Name:     "go",
		Args:     []string{subcommand},
		URI:      uri,
		Resource: resource,
	}
}

// goCmdAt creates a textapi.Command with cursor at (line, char).
func goCmdAt(
	subcommand string, uri workspaceapi.URI, resource textapi.Handler,
	line, char int,
) textapi.Command {
	cmd := goCmd(subcommand, uri, resource)
	cmd.Cursor.Content = term.Coordinates{X: char, Y: line}
	return cmd
}

// collectEditText concatenates the NewText from all edits.
func collectEditText(edits []mockEdit) string {
	var b strings.Builder
	for _, e := range edits {
		b.WriteString(e.NewText)
	}
	return b.String()
}

// mockNotification records a single notification.
type mockNotification struct {
	Level   browserapi.NotificationLevel
	Message string
}

// mockNotifications implements browserapi.Notifications for tests.
type mockNotifications struct {
	mu       sync.Mutex
	messages []mockNotification
}

var _ browserapi.Notifications = (*mockNotifications)(nil)

func (m *mockNotifications) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, mockNotification{
		Level:   level,
		Message: fmt.Sprintf(msg, args...),
	})
	return "", nil
}

func (m *mockNotifications) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return m.Notify(level, msg, args...)
}

func (m *mockNotifications) UpdateNotificationProgress(
	_, _ string, _, _ int64,
) error {
	return nil
}

func (m *mockNotifications) getMessages() []mockNotification {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mockNotification{}, m.messages...)
}

func (m *mockNotifications) hasMessage(substr string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range m.messages {
		if strings.Contains(msg.Message, substr) {
			return true
		}
	}
	return false
}

// mockEdit records a single edit call.
type mockEdit struct {
	Start   term.Coordinates
	End     term.Coordinates
	NewText string
}

// mockCellEditor implements textapi.CellEditor for tests.
type mockCellEditor struct {
	mu    sync.Mutex
	edits []mockEdit
}

var _ textapi.CellEditor = (*mockCellEditor)(nil)

func (e *mockCellEditor) Edit(
	_ context.Context, start, end term.Coordinates, str string,
) (term.Coordinates, term.Coordinates, string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.edits = append(e.edits, mockEdit{Start: start, End: end, NewText: str})
	return start, end, "", nil
}

// mockEditor implements textapi.Editor for tests.
type mockEditor struct {
	mu          sync.Mutex
	handlers    map[string]textapi.Handler // URI string -> handler
	cellEditors map[textapi.Handler]*mockCellEditor
}

var _ textapi.Editor = (*mockEditor)(nil)

func newMockEditor() *mockEditor {
	return &mockEditor{
		handlers:    make(map[string]textapi.Handler),
		cellEditors: make(map[textapi.Handler]*mockCellEditor),
	}
}

// Register associates a handler so that Editor(uri) can find it.
func (e *mockEditor) Register(h textapi.Handler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[h.Resource().String()] = h
}

func (e *mockEditor) SubscribeEvents(
	_ []textapi.EventType, _ textapi.EventHandler,
) error {
	return nil
}

func (e *mockEditor) Editor(uri workspaceapi.URI) (textapi.Handler, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if h, ok := e.handlers[uri.String()]; ok {
		return h, nil
	}
	return nil, errors.New("editor not found")
}

func (e *mockEditor) SetLocationList(
	_ textapi.Handler, _ textapi.LocationPriority, _ string, _ textapi.LocationList,
) error {
	return nil
}

func (e *mockEditor) MoveToNextLocation(_ textapi.Handler, _ string) error {
	return nil
}

func (e *mockEditor) MoveToPrevLocation(_ textapi.Handler, _ string) error {
	return nil
}

func (e *mockEditor) Cursor(_ textapi.Handler) (term.Coordinates, error) {
	return term.Coordinates{}, nil
}

func (e *mockEditor) SetCursor(_ textapi.Handler, _ term.Coordinates) error {
	return nil
}

func (e *mockEditor) CellView(_ textapi.Handler) textapi.CellView {
	return nil
}

func (e *mockEditor) CellEditor(h textapi.Handler) textapi.CellEditor {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ce, ok := e.cellEditors[h]; ok {
		return ce
	}
	ce := &mockCellEditor{}
	e.cellEditors[h] = ce
	return ce
}

func (e *mockEditor) SetDefaultAttributes(_ textapi.Handler, _ term.Attributes) error {
	return nil
}

// editsFor returns all edits recorded by the CellEditor for the given handler.
func (e *mockEditor) editsFor(h textapi.Handler) []mockEdit {
	e.mu.Lock()
	ce, ok := e.cellEditors[h]
	e.mu.Unlock()
	if !ok {
		return nil
	}
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return append([]mockEdit{}, ce.edits...)
}

// bufferCellEditor implements textapi.CellEditor and maintains an in-memory
// text buffer, applying edits as a real editor would. This allows tests to
// verify the final document content after a sequence of edits.
type bufferCellEditor struct {
	lines []string
}

var _ textapi.CellEditor = (*bufferCellEditor)(nil)

func newBufferCellEditor(initial string) *bufferCellEditor {
	lines := strings.Split(initial, "\n")
	return &bufferCellEditor{lines: lines}
}

func (b *bufferCellEditor) Edit(
	_ context.Context, start, end term.Coordinates, text string,
) (term.Coordinates, term.Coordinates, string, error) {
	// Clamp coordinates to buffer bounds.
	if start.Y < 0 {
		start.Y = 0
	}
	if start.Y >= len(b.lines) {
		start.Y = len(b.lines) - 1
	}
	if start.X < 0 {
		start.X = 0
	}
	if start.X > len(b.lines[start.Y]) {
		start.X = len(b.lines[start.Y])
	}
	if end.Y < 0 {
		end.Y = 0
	}
	if end.Y >= len(b.lines) {
		end.Y = len(b.lines) - 1
	}
	if end.X < 0 {
		end.X = 0
	}
	if end.X > len(b.lines[end.Y]) {
		end.X = len(b.lines[end.Y])
	}

	// Build the content before and after the replaced range.
	before := b.lines[start.Y][:start.X]
	after := b.lines[end.Y][end.X:]

	// Combine before + new text + after and re-split into lines.
	combined := before + text + after
	newLines := strings.Split(combined, "\n")

	// Replace the affected lines.
	result := make([]string, 0, start.Y+len(newLines)+len(b.lines)-end.Y-1)
	result = append(result, b.lines[:start.Y]...)
	result = append(result, newLines...)
	result = append(result, b.lines[end.Y+1:]...)
	b.lines = result

	return start, end, "", nil
}

func (b *bufferCellEditor) String() string {
	return strings.Join(b.lines, "\n")
}
