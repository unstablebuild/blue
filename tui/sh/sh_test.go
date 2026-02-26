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

package sh

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

type mockHandler struct {
	mu       sync.Mutex
	calls    []repl.Command
	handleFn func(context.Context, repl.Command) (
		iterator.Iterator[component.Responsive], error,
	)
	completeFn func(
		context.Context, string, []string,
	) (iterator.Iterator[string], error)
}

func (m *mockHandler) HandleCommand(
	ctx context.Context, cmd repl.Command,
) (iterator.Iterator[component.Responsive], error) {
	m.mu.Lock()
	m.calls = append(m.calls, cmd)
	m.mu.Unlock()
	if m.handleFn != nil {
		return m.handleFn(ctx, cmd)
	}
	return iterator.FromSlice[component.Responsive](nil), nil
}

func (m *mockHandler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if m.completeFn != nil {
		return m.completeFn(ctx, cmd, args)
	}
	return iterator.FromSlice[string](nil), nil
}

func collectOutput(
	t *testing.T,
	iter iterator.Iterator[component.Responsive],
) ([]string, error) {
	t.Helper()
	ctx := context.Background()
	var lines []string
	for {
		item, ok := iter.Next(ctx)
		if !ok {
			break
		}
		// Resize so that String() returns the correct
		// text (ResponsiveString needs a proper width).
		h := item.Height(pipeWidth)
		if h > 0 {
			item.Resize(pipeWidth, h)
		}
		lines = append(
			lines,
			responsiveToText(item, pipeWidth),
		)
	}
	return lines, iter.Err()
}

func TestHandleCommand(t *testing.T) {
	cases := []struct {
		name               string
		cmd                repl.Command
		handleFn           func(context.Context, repl.Command) (iterator.Iterator[component.Responsive], error)
		wantCalls          []repl.Command
		wantCallsUnordered []repl.Command
		wantOut            []string
		wantErr            bool
		wantIterErr        bool
	}{
		{
			name:    "builtin echo",
			cmd:     repl.Command{Name: "echo", Args: []string{"hello"}},
			wantOut: []string{"hello"},
		},
		{
			name: "custom command",
			cmd:  repl.Command{Name: "mycmd", Args: []string{"arg1", "arg2"}},
			wantCalls: []repl.Command{
				{Name: "mycmd", Args: []string{"arg1", "arg2"}},
			},
		},
		{
			name: "pipeline",
			cmd:  repl.Command{Name: "mycmd", Args: []string{"|", "mycmd2"}},
			handleFn: func(_ context.Context, cmd repl.Command) (
				iterator.Iterator[component.Responsive], error,
			) {
				if cmd.Name == "mycmd" {
					return iterator.FromSlice([]component.Responsive{
						component.NewResponsiveString(
							"piped-data",
							component.StringResponsiveConfig{},
						),
					}), nil
				}
				return iterator.FromSlice[component.Responsive](nil), nil
			},
			// Pipeline commands run concurrently, so use
			// ElementsMatch (order-insensitive).
			wantCallsUnordered: []repl.Command{
				{Name: "mycmd", Args: []string{}},
				{Name: "mycmd2", Args: []string{}},
			},
		},
		{
			name: "semicolons",
			cmd:  repl.Command{Name: "mycmd;", Args: []string{"mycmd2"}},
			wantCalls: []repl.Command{
				{Name: "mycmd", Args: []string{}},
				{Name: "mycmd2", Args: []string{}},
			},
		},
		{
			name:    "variable expansion",
			cmd:     repl.Command{Name: "FOO=bar;", Args: []string{"echo", "$FOO"}},
			wantOut: []string{"bar"},
		},
		{
			name:    "syntax error",
			cmd:     repl.Command{Name: "echo", Args: []string{`"unterminated`}},
			wantErr: true,
		},
		{
			name: "command error",
			cmd:  repl.Command{Name: "mycmd"},
			handleFn: func(_ context.Context, _ repl.Command) (
				iterator.Iterator[component.Responsive], error,
			) {
				return nil, errors.New("boom")
			},
			wantOut:     []string{"boom"},
			wantIterErr: true,
		},
		{
			name: "fallback to next",
			cmd:  repl.Command{Name: "echo", Args: []string{"hello"}},
			handleFn: func(_ context.Context, _ repl.Command) (
				iterator.Iterator[component.Responsive], error,
			) {
				return nil, repl.ErrNotFound
			},
			wantOut: []string{"hello"},
		},
		{
			name: "empty command",
			cmd:  repl.Command{},
		},
		{
			name:    "logical AND with true",
			cmd:     repl.Command{Name: "true", Args: []string{"&&", "echo", "yes"}},
			wantOut: []string{"yes"},
		},
		{
			name:    "logical OR with false",
			cmd:     repl.Command{Name: "false", Args: []string{"||", "echo", "fallback"}},
			wantOut: []string{"fallback"},
		},
		{
			name:    "subshell",
			cmd:     repl.Command{Name: "(echo", Args: []string{"hello)"}},
			wantOut: []string{"hello"},
		},
		{
			name: "multi-line output",
			cmd:  repl.Command{Name: "mycmd"},
			handleFn: func(_ context.Context, _ repl.Command) (
				iterator.Iterator[component.Responsive], error,
			) {
				return iterator.FromSlice([]component.Responsive{
					component.NewResponsiveString("line1", component.StringResponsiveConfig{}),
					component.NewResponsiveString("line2", component.StringResponsiveConfig{}),
					component.NewResponsiveString("line3", component.StringResponsiveConfig{}),
				}), nil
			},
			wantCalls: []repl.Command{
				{Name: "mycmd", Args: []string{}},
			},
			wantOut: []string{"line1", "line2", "line3"},
		},
		{
			name:        "exit status error",
			cmd:         repl.Command{Name: "false"},
			wantIterErr: true,
		},
		{
			name:        "stderr output",
			cmd:         repl.Command{Name: "echo", Args: []string{"err", ">&2"}},
			wantOut:     []string{"err"},
			wantIterErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockHandler{handleFn: tc.handleFn}
			h := New(mock)
			ctx := context.Background()
			iter, err := h.HandleCommand(ctx, tc.cmd)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			defer func() { _ = iter.Close() }()
			out, iterErr := collectOutput(t, iter)
			if tc.wantIterErr {
				assert.Error(t, iterErr)
			} else {
				assert.NoError(t, iterErr)
			}
			if tc.wantOut != nil {
				assert.Equal(t, tc.wantOut, out)
			}
			if tc.wantCalls != nil {
				mock.mu.Lock()
				assert.Equal(t, tc.wantCalls, mock.calls)
				mock.mu.Unlock()
			}
			if tc.wantCallsUnordered != nil {
				mock.mu.Lock()
				assert.ElementsMatch(t, tc.wantCallsUnordered, mock.calls)
				mock.mu.Unlock()
			}
		})
	}
}

func TestCancelStopsCommand(t *testing.T) {
	mock := &mockHandler{
		handleFn: func(_ context.Context, _ repl.Command) (
			iterator.Iterator[component.Responsive], error,
		) {
			return nil, repl.ErrNotFound
		},
	}
	h := New(mock)
	ctx, cancel := context.WithCancel(context.Background())

	// "yes" produces infinite output; the iterator must
	// stop promptly once the context is cancelled.
	iter, err := h.HandleCommand(ctx, repl.Command{
		Name: "yes",
	})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	// Read a few items to prove the command started.
	for range 3 {
		_, ok := iter.Next(ctx)
		require.True(t, ok, "expected output from yes")
	}

	cancel()

	// After cancellation the iterator must drain within a
	// short deadline. If lineWriter.Write never returns an
	// error the subprocess keeps running and this times out.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, ok := iter.Next(context.Background())
			if !ok {
				return
			}
		}
	}()

	select {
	case <-done:
		// OK — iterator stopped.
	case <-time.After(5 * time.Second):
		t.Fatal("iterator did not stop after context cancellation")
	}
}

func TestLineWriterReturnsErrorOnCancelledContext(t *testing.T) {
	ch := make(chan component.Responsive, 8)
	ctx, cancel := context.WithCancel(context.Background())
	w := &lineWriter{ch: ch, ctx: ctx}

	// Write succeeds before cancellation.
	n, err := w.Write([]byte("hello\n"))
	require.NoError(t, err)
	assert.Equal(t, 6, n)

	cancel()

	// Write returns context error after cancellation.
	_, err = w.Write([]byte("world\n"))
	assert.ErrorIs(t, err, context.Canceled)
}

func TestExitStatusError(t *testing.T) {
	mock := &mockHandler{}
	h := New(mock)
	ctx := context.Background()
	iter, err := h.HandleCommand(ctx, repl.Command{
		Name: "false",
	})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	_, iterErr := collectOutput(t, iter)
	require.Error(t, iterErr)
	var exitErr *repl.ExitError
	require.True(t, errors.As(iterErr, &exitErr))
	assert.Equal(t, 1, exitErr.Code)
}

func TestComplete(t *testing.T) {
	called := false
	mock := &mockHandler{
		completeFn: func(
			_ context.Context, cmd string, args []string,
		) (iterator.Iterator[string], error) {
			called = true
			assert.Equal(t, "foo", cmd)
			assert.Equal(t, []string{"bar"}, args)
			return iterator.FromSlice([]string{"baz"}), nil
		},
	}
	h := New(mock)
	iter, err := h.Complete(
		context.Background(), "foo", []string{"bar"},
	)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(context.Background(), iter)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, []string{"baz"}, got)
}

func TestFileCompletion(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "run.sh"), nil, 0755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden"), nil, 0644))

	// Create a fake executable so LookPath finds "mycmd".
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "mycmd"), []byte("#!/bin/sh\n"), 0755))
	t.Setenv("PATH", binDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	mock := &mockHandler{}
	h := New(mock)
	ctx := context.Background()

	cases := []struct {
		name string
		cmd  string
		args []string
		want []string
	}{
		{
			name: "empty prefix lists non-hidden entries",
			cmd:  "mycmd",
			args: []string{""},
			want: []string{"readme.txt", "run.sh", "subdir/"},
		},
		{
			name: "prefix filters entries",
			cmd:  "mycmd",
			args: []string{"r"},
			want: []string{"readme.txt", "run.sh"},
		},
		{
			name: "dot prefix includes hidden files",
			cmd:  "mycmd",
			args: []string{"."},
			want: []string{".hidden"},
		},
		{
			name: "unknown command returns nothing",
			cmd:  "nonexistent",
			args: []string{"r"},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			iter, err := h.Complete(ctx, tc.cmd, tc.args)
			require.NoError(t, err)
			defer func() { _ = iter.Close() }()
			got, err := iterator.ToSlice(ctx, iter)
			require.NoError(t, err)
			if tc.want == nil {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestPathCompletion(t *testing.T) {
	dir := t.TempDir()
	// Create a fake executable.
	fakeCmd := filepath.Join(dir, "fake-cmd")
	err := os.WriteFile(fakeCmd, []byte("#!/bin/sh\n"), 0755)
	require.NoError(t, err)

	// Override PATH to the temp dir.
	t.Setenv("PATH", dir)

	mock := &mockHandler{}
	h := New(mock)
	ctx := context.Background()

	// Should find fake-cmd with prefix "fak".
	iter, err := h.Complete(ctx, "fak", nil)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"fake-cmd"}, got)

	// Nonexistent prefix should return empty.
	iter2, err := h.Complete(ctx, "nonexistent", nil)
	require.NoError(t, err)
	defer func() { _ = iter2.Close() }()
	got2, err := iterator.ToSlice(ctx, iter2)
	require.NoError(t, err)
	assert.Empty(t, got2)
}
