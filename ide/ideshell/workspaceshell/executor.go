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

// Package workspaceshell provides a workspaceapi.Executor wrapper
// that tracks running processes and exposes "ps" and "kill"
// shell commands via ideshell.CommandHandler.
package workspaceshell

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/unstablebuild/blue/ide/ideshell"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

var (
	_ workspaceapi.Executor  = (*Executor)(nil)
	_ ideshell.CommandHandler = (*Executor)(nil)
)

// Executor wraps a workspaceapi.Executor, tracking every
// started process so it can be listed ("ps") or signalled
// ("kill") from a repl.Handler. It implements both
// workspaceapi.Executor and ideshell.CommandHandler.
type Executor struct {
	mu         sync.RWMutex
	underlying workspaceapi.Executor
	processes  map[workspaceapi.Pid]processInfo
	stats      map[cmdKey]*cmdStats
	now        func() time.Time // for testing
}

type processInfo struct {
	pid     workspaceapi.Pid
	path    string
	args    []string
	dir     string
	env     []string
	started time.Time
	key     cmdKey
}

// cmdKey identifies a command by its path and arguments,
// used to aggregate stats across restarts.
type cmdKey string

func makeCmdKey(path string, args []string) cmdKey {
	return cmdKey(path + "\x00" + strings.Join(args, "\x00"))
}

// cmdStats tracks aggregate statistics for a command
// across process restarts.
type cmdStats struct {
	starts  int
	exits   int
	lastErr error
}

// NewExecutor returns an Executor that delegates to
// underlying while tracking running processes.
func NewExecutor(underlying workspaceapi.Executor) *Executor {
	return &Executor{
		underlying: underlying,
		processes:  make(map[workspaceapi.Pid]processInfo),
		stats:      make(map[cmdKey]*cmdStats),
		now:        time.Now,
	}
}

// Start delegates to the underlying executor and tracks the
// started process. The cmd's Watcher is piped so that
// process exit automatically removes the entry.
func (e *Executor) Start(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	ch := make(chan error, 1)
	ours := workspaceapi.ChanProcessWatcher(ch)
	if cmd.Watcher != nil {
		cmd.Watcher = workspaceapi.MultiProcessWatcher(
			cmd.Watcher, ours,
		)
	} else {
		cmd.Watcher = ours
	}

	pid, err := e.underlying.Start(ctx, cmd)
	if err != nil {
		return pid, err
	}

	key := makeCmdKey(cmd.Path, cmd.Args)
	info := processInfo{
		pid:     pid,
		path:    cmd.Path,
		args:    append([]string{}, cmd.Args...),
		dir:     cmd.Dir,
		env:     append([]string{}, cmd.Env...),
		started: e.now(),
		key:     key,
	}
	e.mu.Lock()
	e.processes[pid] = info
	s := e.stats[key]
	if s == nil {
		s = &cmdStats{}
		e.stats[key] = s
	}
	s.starts++
	e.mu.Unlock()

	go func() {
		exitErr := <-ch
		e.mu.Lock()
		if s := e.stats[key]; s != nil {
			s.exits++
			s.lastErr = exitErr
		}
		delete(e.processes, pid)
		e.mu.Unlock()
	}()

	return pid, nil
}

// Signal delegates to the underlying executor.
func (e *Executor) Signal(
	pid workspaceapi.Pid, sig syscall.Signal,
) error {
	return e.underlying.Signal(pid, sig)
}

// Close delegates to the underlying executor.
func (e *Executor) Close() error {
	return e.underlying.Close()
}

// RegisterCommands registers "ps" and "kill" in the given
// registry with the executor as their handler.
func (e *Executor) RegisterCommands(r *ideshell.CommandRegistry) {
	r.Register("ps", "List running processes", e)
	r.Register("kill", "Send a signal to a process", e)
}

// HandleCommand dispatches "ps" and "kill" commands.
func (e *Executor) HandleCommand(
	ctx context.Context, cmd repl.Command,
) (iterator.Iterator[component.Responsive], error) {
	switch cmd.Name {
	case "ps":
		return e.handlePS(), nil
	case "kill":
		return e.handleKill(cmd.Args)
	default:
		return nil, repl.ErrNotFound
	}
}

// Complete returns PID completions for "kill" and nothing
// for "ps".
func (e *Executor) Complete(
	_ context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if cmd != "kill" || len(args) == 0 {
		return iterator.Empty[string](), nil
	}
	prefix := args[len(args)-1]
	e.mu.RLock()
	pids := make([]string, 0, len(e.processes))
	for pid := range e.processes {
		s := strconv.Itoa(int(pid))
		if strings.HasPrefix(s, prefix) {
			pids = append(pids, s)
		}
	}
	e.mu.RUnlock()
	sort.Strings(pids)
	return iterator.FromSlice(pids), nil
}

// Help returns usage information for ps and kill.
func (e *Executor) Help(
	_ context.Context, _ []string,
) (iterator.Iterator[component.Responsive], error) {
	return toLines(
		"Process management commands:",
		"",
		"  ps             List running processes with uptime and restart count",
		"  kill <pid>     Send SIGTERM to a process",
		"  kill -N <pid>  Send signal N to a process",
	), nil
}

type psEntry struct {
	pid      workspaceapi.Pid
	uptime   time.Duration
	restarts int
	command  string
}

func (e *Executor) handlePS() iterator.Iterator[component.Responsive] {
	now := e.now()
	e.mu.RLock()
	entries := make([]psEntry, 0, len(e.processes))
	for _, info := range e.processes {
		cmd := info.path
		if len(info.args) > 0 {
			cmd += " " + strings.Join(info.args, " ")
		}
		restarts := 0
		if s := e.stats[info.key]; s != nil {
			restarts = s.starts - 1
		}
		entries = append(entries, psEntry{
			pid:      info.pid,
			uptime:   now.Sub(info.started),
			restarts: restarts,
			command:  cmd,
		})
	}
	e.mu.RUnlock()

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].pid < entries[j].pid
	})

	lines := make([]component.Responsive, 0, len(entries)+1)
	lines = append(lines, toResponsive(
		fmt.Sprintf("  %-8s %-10s %-10s %s", "PID", "UPTIME", "RESTARTS", "COMMAND"),
	))
	for _, ent := range entries {
		lines = append(lines, toResponsive(
			fmt.Sprintf("  %-8d %-10s %-10d %s",
				ent.pid, formatDuration(ent.uptime),
				ent.restarts, ent.command),
		))
	}
	return iterator.FromSlice(lines)
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		m := int(d.Minutes())
		s := int(d.Seconds()) - m*60
		return fmt.Sprintf("%dm%ds", m, s)
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) - h*60
		return fmt.Sprintf("%dh%dm", h, m)
	default:
		days := int(d.Hours()) / 24
		h := int(d.Hours()) - days*24
		return fmt.Sprintf("%dd%dh", days, h)
	}
}

func (e *Executor) handleKill(
	args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: kill [-signal] <pid>")
	}

	sig := syscall.SIGTERM
	pidStr := args[0]

	if strings.HasPrefix(pidStr, "-") && len(args) > 1 {
		n, err := strconv.Atoi(pidStr[1:])
		if err != nil {
			return nil, fmt.Errorf("invalid signal: %s", pidStr[1:])
		}
		sig = syscall.Signal(n)
		pidStr = args[1]
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid pid: %s", pidStr)
	}

	if err := e.Signal(workspaceapi.Pid(pid), sig); err != nil {
		return nil, err
	}

	return iterator.Empty[component.Responsive](), nil
}

func toLines(ss ...string) iterator.Iterator[component.Responsive] {
	out := make([]component.Responsive, len(ss))
	for i, s := range ss {
		out[i] = toResponsive(s)
	}
	return iterator.FromSlice(out)
}

func toResponsive(s string) component.Responsive {
	return component.NewResponsiveString(
		s, component.StringResponsiveConfig{},
	)
}
