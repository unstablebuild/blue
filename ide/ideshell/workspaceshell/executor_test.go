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

package workspaceshell

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const testWidth = 200

// mockExecutor is a minimal workspaceapi.Executor for testing.
type mockExecutor struct {
	mu       sync.Mutex
	nextPid  workspaceapi.Pid
	starts   []workspaceapi.Cmd
	signals  []signalCall
	watchers map[workspaceapi.Pid]workspaceapi.ProcessWatcher
	startErr error
}

type signalCall struct {
	pid workspaceapi.Pid
	sig syscall.Signal
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		nextPid:  1,
		watchers: make(map[workspaceapi.Pid]workspaceapi.ProcessWatcher),
	}
}

func (m *mockExecutor) Start(
	_ context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return 0, m.startErr
	}
	pid := m.nextPid
	m.nextPid++
	m.starts = append(m.starts, cmd)
	m.watchers[pid] = cmd.Watcher
	return pid, nil
}

func (m *mockExecutor) Signal(
	pid workspaceapi.Pid, sig syscall.Signal,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signals = append(m.signals, signalCall{pid: pid, sig: sig})
	return nil
}

func (m *mockExecutor) Close() error { return nil }

// exitProcess simulates a process exiting successfully.
func (m *mockExecutor) exitProcess(pid workspaceapi.Pid) {
	m.exitProcessWithError(pid, nil)
}

// exitProcessWithError simulates a process exiting with
// the given error.
func (m *mockExecutor) exitProcessWithError(
	pid workspaceapi.Pid, err error,
) {
	m.mu.Lock()
	w := m.watchers[pid]
	m.mu.Unlock()
	if w != nil {
		w.WatchProcess() <- err
	}
}

func collectText(
	t *testing.T,
	iter iterator.Iterator[component.Responsive],
) []string {
	t.Helper()
	ctx := context.Background()
	var lines []string
	for {
		item, ok := iter.Next(ctx)
		if !ok {
			break
		}
		h := item.Height(testWidth)
		if h <= 0 {
			continue
		}
		w := term.NewStringWriter(testWidth, h)
		item.Resize(testWidth, h)
		item.Draw(w)
		_ = w.Flush()
		lines = append(lines, w.String())
	}
	require.NoError(t, iter.Err())
	return lines
}

func TestStartTracksProcess(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{
		Path: "/usr/bin/gopls",
		Args: []string{"-rpc.trace"},
		Dir:  "/home/user",
	})
	require.NoError(t, err)
	assert.Equal(t, workspaceapi.Pid(1), pid)

	// Process should appear in ps output.
	iter := exec.handlePS()
	defer func() { _ = iter.Close() }()
	out := collectText(t, iter)
	require.Len(t, out, 2) // header + 1 process
	assert.Contains(t, out[1], "gopls")
	assert.Contains(t, out[1], "-rpc.trace")
}

func fixedTime(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestStartPipesOriginalWatcher(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	origCh := make(chan error, 1)
	origWatcher := workspaceapi.ChanProcessWatcher(origCh)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{
		Path:    "/usr/bin/ls",
		Watcher: origWatcher,
	})
	require.NoError(t, err)

	// Simulate process exit.
	mock.exitProcess(pid)

	// Original watcher should also be notified.
	exitErr := <-origCh
	assert.NoError(t, exitErr)
}

func TestExitRemovesProcess(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{
		Path: "/usr/bin/ls",
	})
	require.NoError(t, err)

	// Simulate exit.
	mock.exitProcess(pid)

	// Wait for goroutine to clean up by checking ps.
	assert.Eventually(t, func() bool {
		exec.mu.RLock()
		defer exec.mu.RUnlock()
		return len(exec.processes) == 0
	}, 1e9, 1e6)
}

func TestStartError(t *testing.T) {
	mock := newMockExecutor()
	mock.startErr = errors.New("boom")
	exec := NewExecutor(mock)

	ctx := context.Background()
	_, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/x"})
	assert.Error(t, err)

	// No process tracked.
	exec.mu.RLock()
	assert.Empty(t, exec.processes)
	exec.mu.RUnlock()
}

func TestSignalDelegates(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	err := exec.Signal(42, syscall.SIGTERM)
	require.NoError(t, err)

	mock.mu.Lock()
	assert.Equal(t, []signalCall{{pid: 42, sig: syscall.SIGTERM}}, mock.signals)
	mock.mu.Unlock()
}

func TestHandleCommandPS(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	_, err := exec.Start(ctx, workspaceapi.Cmd{
		Path: "/usr/bin/gopls",
		Args: []string{"-rpc.trace"},
	})
	require.NoError(t, err)
	_, err = exec.Start(ctx, workspaceapi.Cmd{
		Path: "/usr/bin/bash",
	})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{Name: "ps"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	require.Len(t, out, 3) // header + 2 processes
	assert.Contains(t, out[0], "PID")
	assert.Contains(t, out[0], "UPTIME")
	assert.Contains(t, out[0], "RESTARTS")
	assert.Contains(t, out[0], "COMMAND")
	assert.Contains(t, out[1], "gopls")
	assert.Contains(t, out[2], "bash")
}

func TestHandleCommandKill(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/sleep"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "kill",
		Args: []string{strconv.Itoa(int(pid))},
	})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	mock.mu.Lock()
	require.Len(t, mock.signals, 1)
	assert.Equal(t, pid, mock.signals[0].pid)
	assert.Equal(t, syscall.SIGTERM, mock.signals[0].sig)
	mock.mu.Unlock()
}

func TestHandleCommandKillWithSignal(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/sleep"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "kill",
		Args: []string{"-9", strconv.Itoa(int(pid))},
	})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	mock.mu.Lock()
	require.Len(t, mock.signals, 1)
	assert.Equal(t, pid, mock.signals[0].pid)
	assert.Equal(t, syscall.SIGKILL, mock.signals[0].sig)
	mock.mu.Unlock()
}

func TestHandleCommandKillNoArgs(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{
		Name: "kill",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usage")
}

func TestHandleCommandKillInvalidPid(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{
		Name: "kill",
		Args: []string{"abc"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pid")
}

func TestHandleCommandKillInvalidSignal(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{
		Name: "kill",
		Args: []string{"-xyz", "1"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signal")
}

func TestHandleCommandUnknown(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{Name: "nope"})
	assert.True(t, errors.Is(err, repl.ErrNotFound))
}

func TestCompletePids(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	// Start processes with PIDs 1, 2, 3.
	for range 3 {
		_, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/x"})
		require.NoError(t, err)
	}

	// Complete "kill 1" → should return "1".
	iter, err := exec.Complete(ctx, "kill", []string{"1"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, got)

	// Complete "kill " with empty prefix → all PIDs.
	iter2, err := exec.Complete(ctx, "kill", []string{""})
	require.NoError(t, err)
	defer func() { _ = iter2.Close() }()
	got2, err := iterator.ToSlice(ctx, iter2)
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2", "3"}, got2)
}

func TestCompletePSReturnsEmpty(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	iter, err := exec.Complete(ctx, "ps", []string{""})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestHelp(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	iter, err := exec.Help(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	assert.NotEmpty(t, out)
	// Should mention both ps and kill.
	combined := ""
	for _, line := range out {
		combined += line
	}
	assert.Contains(t, combined, "ps")
	assert.Contains(t, combined, "kill")
}

func TestPSSortedByPid(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	// Start 3 processes.
	for _, path := range []string{"/c", "/a", "/b"} {
		_, err := exec.Start(ctx, workspaceapi.Cmd{Path: path})
		require.NoError(t, err)
	}

	iter := exec.handlePS()
	defer func() { _ = iter.Close() }()
	out := collectText(t, iter)
	require.Len(t, out, 4) // header + 3

	// PIDs should be 1, 2, 3 in order.
	assert.Contains(t, out[1], "/c") // pid 1
	assert.Contains(t, out[2], "/a") // pid 2
	assert.Contains(t, out[3], "/b") // pid 3
}

func TestPSShowsUptime(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exec.now = fixedTime(started)

	ctx := context.Background()
	_, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/x"})
	require.NoError(t, err)

	// Advance time by 5 minutes and 32 seconds.
	exec.now = fixedTime(started.Add(5*time.Minute + 32*time.Second))

	iter := exec.handlePS()
	defer func() { _ = iter.Close() }()
	out := collectText(t, iter)
	require.Len(t, out, 2)
	assert.Contains(t, out[1], "5m32s")
}

func TestPSShowsRestartCount(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	cmd := workspaceapi.Cmd{
		Path: "/usr/bin/gopls",
		Args: []string{"-rpc.trace"},
	}

	// Start, exit, start again (restart count = 1).
	pid1, err := exec.Start(ctx, cmd)
	require.NoError(t, err)
	mock.exitProcess(pid1)
	assert.Eventually(t, func() bool {
		exec.mu.RLock()
		defer exec.mu.RUnlock()
		return len(exec.processes) == 0
	}, 1e9, 1e6)

	_, err = exec.Start(ctx, cmd)
	require.NoError(t, err)

	iter := exec.handlePS()
	defer func() { _ = iter.Close() }()
	out := collectText(t, iter)
	require.Len(t, out, 2)
	// Restart count should be 1 (started twice, so restarts = starts - 1 = 1).
	assert.Contains(t, out[1], "1")
	assert.Contains(t, out[1], "gopls")
}

func TestExitTracksStats(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	cmd := workspaceapi.Cmd{Path: "/bin/x", Args: []string{"a"}}

	pid, err := exec.Start(ctx, cmd)
	require.NoError(t, err)

	key := makeCmdKey(cmd.Path, cmd.Args)
	exec.mu.RLock()
	assert.Equal(t, 1, exec.stats[key].starts)
	assert.Equal(t, 0, exec.stats[key].exits)
	exec.mu.RUnlock()

	// Simulate exit with error.
	mock.exitProcessWithError(pid, errors.New("segfault"))

	assert.Eventually(t, func() bool {
		exec.mu.RLock()
		defer exec.mu.RUnlock()
		return exec.stats[key].exits == 1
	}, 1e9, 1e6)

	exec.mu.RLock()
	assert.EqualError(t, exec.stats[key].lastErr, "segfault")
	exec.mu.RUnlock()
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{5*time.Minute + 32*time.Second, "5m32s"},
		{2*time.Hour + 15*time.Minute, "2h15m"},
		{36 * time.Hour, "1d12h"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, formatDuration(tc.d), "duration=%v", tc.d)
	}
}
