// Copyright 2026 Unstable Build, LLC.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package idedebug

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-dap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/debugapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func TestE2E(t *testing.T) {
	t.Parallel()
	dlvBin := findDlv(t)
	tmpDir := setupTestWorkspace(t, "go")

	uri := makeURI(t, "file://"+tmpDir)
	mainPath := filepath.Join(tmpDir, "main.go")

	// Line number for "sum := Add(x, y)" in
	// go/main.go (after the license header).
	const breakpointLine = 27

	tests := []struct {
		name string
		fn   func(t *testing.T, mgr *Manager, events <-chan dap.EventMessage)
	}{
		{
			name: "Threads",
			fn: func(t *testing.T, mgr *Manager, _ <-chan dap.EventMessage) {
				threads, err := mgr.Threads(
					t.Context(),
				)
				require.NoError(t, err)
				require.GreaterOrEqual(t, len(threads), 1)
				mainThread := findMainThread(t, threads)
				assert.Contains(t, mainThread.Name, "main.main")
			},
		},
		{
			name: "StackTrace",
			fn: func(t *testing.T, mgr *Manager, _ <-chan dap.EventMessage) {
				threads, err := mgr.Threads(
					t.Context(),
				)
				require.NoError(t, err)
				mainThread := findMainThread(t, threads)

				st, err := mgr.StackTrace(
					t.Context(),
					&dap.StackTraceArguments{
						ThreadId: mainThread.Id,
					},
				)
				require.NoError(t, err)
				require.GreaterOrEqual(t,
					len(st.StackFrames), 1,
				)
				frame := st.StackFrames[0]
				assert.Equal(t, "main.main", frame.Name)
				require.NotNil(t, frame.Source)
				assert.Equal(t, "main.go", frame.Source.Name)
				assert.Equal(t, mainPath, frame.Source.Path)
				assert.Equal(t, breakpointLine, frame.Line)
			},
		},
		{
			name: "Scopes",
			fn: func(t *testing.T, mgr *Manager, _ <-chan dap.EventMessage) {
				threads, err := mgr.Threads(
					t.Context(),
				)
				require.NoError(t, err)
				mainThread := findMainThread(t, threads)

				st, err := mgr.StackTrace(
					t.Context(),
					&dap.StackTraceArguments{
						ThreadId: mainThread.Id,
					},
				)
				require.NoError(t, err)
				require.GreaterOrEqual(t,
					len(st.StackFrames), 1,
				)

				scopes, err := mgr.Scopes(
					t.Context(),
					&dap.ScopesArguments{
						FrameId: st.StackFrames[0].Id,
					},
				)
				require.NoError(t, err)
				require.GreaterOrEqual(t, len(scopes), 1)
				// Find Locals scope
				var localsScope *dap.Scope
				for i := range scopes {
					if scopes[i].Name == "Locals" {
						localsScope = &scopes[i]
						break
					}
				}
				require.NotNil(t, localsScope, "Locals scope not found")
				assert.Greater(t, localsScope.VariablesReference, 0)
			},
		},
		{
			name: "Variables",
			fn: func(t *testing.T, mgr *Manager, _ <-chan dap.EventMessage) {
				threads, err := mgr.Threads(
					t.Context(),
				)
				require.NoError(t, err)
				mainThread := findMainThread(t, threads)

				st, err := mgr.StackTrace(
					t.Context(),
					&dap.StackTraceArguments{
						ThreadId: mainThread.Id,
					},
				)
				require.NoError(t, err)
				require.GreaterOrEqual(t,
					len(st.StackFrames), 1,
				)

				scopes, err := mgr.Scopes(
					t.Context(),
					&dap.ScopesArguments{
						FrameId: st.StackFrames[0].Id,
					},
				)
				require.NoError(t, err)
				// Find Locals scope
				var localsRef int
				for _, s := range scopes {
					if s.Name == "Locals" {
						localsRef = s.VariablesReference
						break
					}
				}
				require.Greater(t, localsRef, 0)

				vars, err := mgr.Variables(
					t.Context(),
					&dap.VariablesArguments{
						VariablesReference: localsRef,
					},
				)
				require.NoError(t, err)
				require.GreaterOrEqual(t, len(vars), 2)

				// Build a map for easier lookup
				varMap := make(map[string]dap.Variable)
				for _, v := range vars {
					varMap[v.Name] = v
				}

				xVar, ok := varMap["x"]
				require.True(t, ok, "variable x not found")
				assert.Equal(t, "10", xVar.Value)

				yVar, ok := varMap["y"]
				require.True(t, ok, "variable y not found")
				assert.Equal(t, "20", yVar.Value)
			},
		},
		{
			name: "Evaluate",
			fn: func(t *testing.T, mgr *Manager, _ <-chan dap.EventMessage) {
				threads, err := mgr.Threads(
					t.Context(),
				)
				require.NoError(t, err)
				mainThread := findMainThread(t, threads)

				st, err := mgr.StackTrace(
					t.Context(),
					&dap.StackTraceArguments{
						ThreadId: mainThread.Id,
					},
				)
				require.NoError(t, err)
				require.GreaterOrEqual(t,
					len(st.StackFrames), 1,
				)

				result, err := mgr.Evaluate(
					t.Context(),
					&dap.EvaluateArguments{
						Expression: "x + y",
						FrameId:    st.StackFrames[0].Id,
					},
				)
				require.NoError(t, err)
				assert.Equal(t, "30", result.Result)
			},
		},
		{
			name: "ContinueAndTerminate",
			fn: func(t *testing.T, mgr *Manager, events <-chan dap.EventMessage) {
				resp, err := mgr.Continue(
					t.Context(),
					&dap.ContinueArguments{
						ThreadId: 1,
					},
				)
				require.NoError(t, err)
				assert.Equal(t,
					&dap.ContinueResponseBody{
						AllThreadsContinued: true,
					}, resp,
				)
				waitForEvent(t, events, "terminated")
			},
		},
	}

	t.Run("lazy init via Handle EventTypeOpen",
		func(t *testing.T) {
			t.Parallel()
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					executor := newTestExecutor(t)
					eventSub := newTestEventSubscriber()
					mgr := New(
						uri,
						executor,
						&stubPkgManager{bin: dlvBin},
						Config{MaxRetries: 1, EventSubscriber: eventSub},
					)

					mainWSURI, err :=
						workspaceapi.ParseURI(
							"file://" + mainPath,
						)
					require.NoError(t, err)

					mgr.Handle(
						context.Background(),
						textapi.Event{
							Type: textapi.EventTypeOpen,
							URI:  mainWSURI,
						},
					)

					setupDebugSession(t, mgr, eventSub.events, tmpDir, mainPath, breakpointLine)
					tt.fn(t, mgr, eventSub.events)
					require.NoError(t, mgr.Close())
				})
			}
		})

	t.Run("explicit init via Initialize",
		func(t *testing.T) {
			t.Parallel()
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					executor := newTestExecutor(t)
					eventSub := newTestEventSubscriber()
					mgr := New(
						uri,
						executor,
						&stubPkgManager{bin: dlvBin},
						Config{MaxRetries: 1, EventSubscriber: eventSub},
					)

					initOpts, err := json.Marshal(map[string]any{
						"langID":  "go",
						"command": dlvBin + " dap --listen={addr}",
					})
					require.NoError(t, err)

					caps, err := mgr.Initialize(
						context.Background(),
						&debugapi.InitializeRequestArguments{
							InitializeOptions: initOpts,
						},
					)
					require.NoError(t, err)
					require.NotNil(t, caps)

					setupDebugSession(t, mgr, eventSub.events, tmpDir, mainPath, breakpointLine)
					tt.fn(t, mgr, eventSub.events)
					require.NoError(t, mgr.Close())
				})
			}
		})

	t.Run("explicit init via InitializeOptions",
		func(t *testing.T) {
			t.Parallel()
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					executor := newTestExecutor(t)
					eventSub := newTestEventSubscriber()
					mgr := New(
						uri,
						executor,
						&stubPkgManager{bin: dlvBin},
						Config{MaxRetries: 1, EventSubscriber: eventSub},
					)

					initOpts, err := json.Marshal(map[string]any{
						"langID":  "go",
						"command": dlvBin + " dap --listen={addr}",
					})
					require.NoError(t, err)

					caps, err := mgr.Initialize(
						context.Background(),
						&debugapi.InitializeRequestArguments{
							InitializeOptions: initOpts,
						},
					)
					require.NoError(t, err)
					require.NotNil(t, caps)

					setupDebugSession(t, mgr, eventSub.events, tmpDir, mainPath, breakpointLine)
					tt.fn(t, mgr, eventSub.events)
					require.NoError(t, mgr.Close())
				})
			}
		})
}

func TestE2E_NoReadErrorWarnings(t *testing.T) {
	t.Parallel()
	dlvBin := findDlv(t)
	tmpDir := setupTestWorkspace(t, "go")

	uri := makeURI(t, "file://"+tmpDir)
	mainPath := filepath.Join(tmpDir, "main.go")
	const breakpointLine = 27

	// Install a warn-capturing slog handler.
	// Use a TextHandler writing to stderr as the inner
	// handler instead of slog.Default().Handler(). The
	// default handler writes through the log package, and
	// slog.SetDefault redirects the log package through our
	// handler — creating a re-entrant deadlock on the log
	// mutex.
	h := &warnHandler{
		inner: slog.NewTextHandler(os.Stderr, nil),
	}
	orig := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(orig) })

	executor := newTestExecutor(t)
	eventSub := newTestEventSubscriber()
	mgr := New(
		uri,
		executor,
		&stubPkgManager{bin: dlvBin},
		Config{MaxRetries: 1, EventSubscriber: eventSub},
	)

	initOpts, err := json.Marshal(map[string]any{
		"langID":  "go",
		"command": dlvBin + " dap --listen={addr}",
	})
	require.NoError(t, err)

	caps, err := mgr.Initialize(
		context.Background(),
		&debugapi.InitializeRequestArguments{
			InitializeOptions: initOpts,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, caps)

	setupDebugSession(t, mgr, eventSub.events, tmpDir, mainPath, breakpointLine)

	// Continue so the program finishes.
	_, err = mgr.Continue(
		t.Context(),
		&dap.ContinueArguments{ThreadId: 1},
	)
	require.NoError(t, err)
	waitForEvent(t, eventSub.events, "terminated")

	require.NoError(t, mgr.Close())

	// Assert no spurious "read error" warnings.
	warnings := h.readErrors()
	assert.Empty(t, warnings, "unexpected 'read error' warnings: %v", warnings)
}

// warnHandler is an slog.Handler that delegates to an inner
// handler and records all LevelWarn+ records whose message is
// "read error".
type warnHandler struct {
	inner   slog.Handler
	mu      sync.Mutex
	records []slog.Record
}

func (w *warnHandler) Enabled(_ context.Context, l slog.Level) bool {
	return true
}

func (w *warnHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn && r.Message == "read error" {
		w.mu.Lock()
		w.records = append(w.records, r)
		w.mu.Unlock()
	}
	return w.inner.Handle(ctx, r)
}

func (w *warnHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &warnHandler{inner: w.inner.WithAttrs(attrs)}
}

func (w *warnHandler) WithGroup(name string) slog.Handler {
	return &warnHandler{inner: w.inner.WithGroup(name)}
}

func (w *warnHandler) readErrors() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var msgs []string
	for _, r := range w.records {
		msgs = append(msgs, r.Message)
	}
	return msgs
}

// TestE2E_LaunchErrorSurfaced verifies that when Launch
// fails (e.g. because the program path doesn't exist), the
// error is captured and surfaced by ConfigurationDone
// instead of the generic "No debug session started" message.
func TestE2E_LaunchErrorSurfaced(t *testing.T) {
	t.Parallel()
	dlvBin := findDlv(t)
	tmpDir := setupTestWorkspace(t, "go")

	uri := makeURI(t, "file://"+tmpDir)

	executor := newTestExecutor(t)
	eventSub := newTestEventSubscriber()
	mgr := New(
		uri,
		executor,
		&stubPkgManager{bin: dlvBin},
		Config{MaxRetries: 1, EventSubscriber: eventSub},
	)

	initOpts, err := json.Marshal(map[string]any{
		"langID":  "go",
		"command": dlvBin + " dap --listen={addr}",
	})
	require.NoError(t, err)

	caps, err := mgr.Initialize(
		t.Context(),
		&debugapi.InitializeRequestArguments{
			InitializeOptions: initOpts,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, caps)

	// Launch with a nonexistent program. writeRequest
	// (fire-and-forget) succeeds because the TCP write
	// works, but dlv will fail to build the program.
	err = mgr.Launch(t.Context(), debugapi.LaunchRequestArguments{
		Program: "/nonexistent/path/to/program",
	})
	require.NoError(t, err, "Launch is fire-and-forget, should not error")

	// ConfigurationDone should surface the actual launch
	// failure instead of the generic "No debug session
	// started" from dlv.
	err = mgr.ConfigurationDone(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to launch",
		"expected the build error from dlv, got: %v", err)

	require.NoError(t, mgr.Close())
}

// setupDebugSession performs the DAP launch sequence:
// Launch -> wait for "initialized" -> SetBreakpoints ->
// ConfigurationDone -> wait for "stopped".
// In DAP, the initialized event arrives during Launch
// processing, and LaunchResponse arrives only after
// ConfigurationDone.
func setupDebugSession(
	t *testing.T,
	mgr *Manager,
	events <-chan dap.EventMessage,
	program string,
	mainPath string,
	breakpointLine int,
) {
	t.Helper()
	ctx := t.Context()

	err := launchWithRetry(t, mgr, debugapi.LaunchRequestArguments{
		Program: program,
	})
	require.NoError(t, err)

	// In DAP, the initialized event is sent by the debug
	// adapter during Launch, before the LaunchResponse.
	waitForEvent(t, events, "initialized")

	bps, err := mgr.SetBreakpoints(
		ctx,
		&dap.SetBreakpointsArguments{
			Source: dap.Source{
				Path: mainPath,
			},
			Breakpoints: []dap.SourceBreakpoint{
				{Line: breakpointLine},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, bps, 1)
	assert.Equal(t, dap.Breakpoint{
		Id:       bps[0].Id,
		Verified: true,
		Source: &dap.Source{
			Name: "main.go",
			Path: mainPath,
		},
		Line: breakpointLine,
	}, bps[0])

	err = mgr.ConfigurationDone(ctx)
	require.NoError(t, err)

	waitForEvent(t, events, "stopped")
}

// launchWithRetry retries Launch to handle the case where
// the server was started asynchronously via Handle and may
// not be in the server map yet.
func launchWithRetry(
	t *testing.T,
	mgr *Manager,
	args debugapi.LaunchRequestArguments,
) error {
	t.Helper()
	const maxRetries = 50
	for i := range maxRetries {
		err := mgr.Launch(t.Context(), args)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrNoServer) || i == maxRetries-1 {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func findDlv(t *testing.T) string {
	t.Helper()
	dlvBin, err := exec.LookPath("dlv")
	if err != nil {
		envs := []string{
			filepath.Join(os.Getenv("HOME"), ".rune", "bin", "dlv"),
			filepath.Join(os.Getenv("HOME"), "go", "bin", "dlv"),
		}
		for _, p := range envs {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		t.Skip("dlv not found, skipping e2e test")
	}
	return dlvBin
}

func setupTestWorkspace(
	t *testing.T, testdataDir string,
) string {
	t.Helper()
	tmpDir := t.TempDir()

	if testdataDir == "" {
		return tmpDir
	}

	entries, err := os.ReadDir(testdataDir)
	require.NoError(t, err)
	for _, e := range entries {
		src := filepath.Join(testdataDir, e.Name())
		dst := filepath.Join(tmpDir, e.Name())
		data, err := os.ReadFile(src)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(
			dst, data, 0644,
		))
	}
	return tmpDir
}

// findMainThread returns the thread running main.main.
// dlv returns multiple threads (runtime goroutines), so
// we need to find the one we care about.
func findMainThread(
	t *testing.T, threads []dap.Thread,
) dap.Thread {
	t.Helper()
	for _, th := range threads {
		if strings.Contains(th.Name, "main.main") {
			return th
		}
	}
	t.Fatalf("main.main thread not found in %v", threads)
	return dap.Thread{}
}

func waitForEvent(
	t *testing.T,
	ch <-chan dap.EventMessage,
	eventType string,
) {
	t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.GetEvent().Event == eventType {
				return
			}
		case <-timeout:
			t.Fatalf(
				"timeout waiting for %s event",
				eventType,
			)
		}
	}
}

func makeURI(
	t *testing.T, uri string,
) workspaceapi.URI {
	ret, err := workspaceapi.ParseURI(uri)
	require.NoError(t, err)
	return ret
}

var _ PkgManager = (*stubPkgManager)(nil)

type stubPkgManager struct {
	bin string
}

var _ EventSubscriber = (*testEventSubscriber)(nil)

type testEventSubscriber struct {
	events chan dap.EventMessage
}

func newTestEventSubscriber() *testEventSubscriber {
	return &testEventSubscriber{
		events: make(chan dap.EventMessage, 64),
	}
}

func (s *testEventSubscriber) OnEvent(ev dap.EventMessage) {
	select {
	case s.events <- ev:
	default:
		// drop if full
	}
}

func (p *stubPkgManager) LibDir(
	_ context.Context, _ string,
) (iterator.Iterator[string], error) {
	return iterator.FromSlice(
		[]string{p.bin},
	), nil
}

var _ schemeapi.Executor = (*testExecutor)(nil)

type testExecutor struct {
	mu        sync.Mutex
	wg        sync.WaitGroup
	processes map[int]*os.Process
	nextPid   int
}

func newTestExecutor(t *testing.T) *testExecutor {
	e := &testExecutor{
		processes: make(map[int]*os.Process),
	}
	t.Cleanup(func() {
		e.mu.Lock()
		for _, p := range e.processes {
			_ = p.Kill()
		}
		e.mu.Unlock()
		e.wg.Wait()
	})
	return e
}

func (e *testExecutor) StartCommand(
	_ context.Context,
	cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	c := exec.Command(cmd.Path, cmd.Args...)
	if cmd.Dir != "" {
		c.Dir = cmd.Dir
	}
	c.Env = cmd.Env
	c.Stdin = cmd.Stdin
	c.Stdout = cmd.Stdout
	if cmd.Stderr != nil {
		c.Stderr = cmd.Stderr
	}

	if err := c.Start(); err != nil {
		return 0, fmt.Errorf(
			"start command: %w", err,
		)
	}

	e.mu.Lock()
	e.nextPid++
	pid := workspaceapi.Pid(e.nextPid)
	e.processes[int(pid)] = c.Process
	e.mu.Unlock()

	if cmd.Watcher != nil {
		ch := cmd.Watcher.WatchProcess()
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			err := c.Wait()
			if ch != nil {
				ch <- err
			}
		}()
	}

	return pid, nil
}

func (e *testExecutor) Signal(
	pid workspaceapi.Pid, sig syscall.Signal,
) error {
	e.mu.Lock()
	p, ok := e.processes[int(pid)]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf(
			"process not found: %d", pid,
		)
	}
	return p.Signal(sig)
}

func (e *testExecutor) Close() error {
	return nil
}
