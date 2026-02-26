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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// New returns a repl.CommandHandler that interprets
// shell syntax (pipes, semicolons, variables, etc.)
// using mvdan/sh and delegates actual command execution
// to the underlying handler.
func New(underlying repl.CommandHandler) repl.CommandHandler {
	return &commandHandler{underlying: underlying}
}

const pipeWidth = 200

type commandHandler struct {
	underlying repl.CommandHandler
}

// HandleCommand parses the command line as shell syntax
// and executes it via mvdan/sh, delegating non-builtin
// commands to the underlying handler. Output is streamed
// via a channel-backed iterator.
func (h *commandHandler) HandleCommand(
	ctx context.Context, cmd repl.Command,
) (iterator.Iterator[component.Responsive], error) {
	line := reconstructLine(cmd)
	if line == "" {
		return iterator.FromSlice[component.Responsive](nil), nil
	}

	file, err := syntax.NewParser().Parse(
		strings.NewReader(line), "",
	)
	if err != nil {
		return nil, err
	}

	ch := make(chan component.Responsive, 64)
	outW := &lineWriter{ch: ch, ctx: ctx}
	errW := &lineWriter{ch: ch, ctx: ctx}

	runner, err := interp.New(
		interp.StdIO(nil, outW, errW),
		interp.ExecHandlers(h.execMiddleware),
		interp.Interactive(true),
	)
	if err != nil {
		return nil, err
	}

	var runErr error
	go func() {
		defer close(ch)
		runErr = runner.Run(ctx, file)
		outW.flush()
		errW.flush()
	}()

	return iterator.FromFunc(func(ctx context.Context) (component.Responsive, bool, error) {
		select {
		case item, ok := <-ch:
			if !ok {
				if exit, ok := runErr.(interp.ExitStatus); ok && int(exit) != 0 {
					return nil, false, &repl.ExitError{Code: int(exit)}
				}
				if runErr != nil && !errors.Is(runErr, context.Canceled) {
					return nil, false, runErr
				}
				return nil, false, nil
			}
			return item, true, nil
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
	}, func() error { return nil }), nil
}

// Complete delegates to the underlying handler. When
// completing a command name (args == nil) and the
// underlying returns no results, it falls back to
// scanning $PATH directories for matching executables.
// When completing arguments (args != nil) and the
// underlying returns no results, it falls back to
// file-based completion if cmd is a known executable.
func (h *commandHandler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	iter, err := h.underlying.Complete(ctx, cmd, args)
	if err != nil {
		return nil, err
	}
	if args == nil {
		// Command name completion — PATH fallback.
		iter, empty := iterator.IsEmpty(ctx, iter)
		if !empty {
			return iter, nil
		}
		return iterator.FromSlice(pathCompletions(cmd)), nil
	}
	// Argument completion — file fallback when the
	// underlying returns nothing and cmd is on PATH.
	iter, empty := iterator.IsEmpty(ctx, iter)
	if !empty {
		return iter, nil
	}
	if _, err := exec.LookPath(cmd); err != nil {
		return iter, nil
	}
	var prefix string
	if len(args) > 0 {
		prefix = args[len(args)-1]
	}
	return iterator.FromSlice(fileCompletions(prefix)), nil
}

func (h *commandHandler) execMiddleware(
	next interp.ExecHandlerFunc,
) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		hc := interp.HandlerCtx(ctx)
		cmd := repl.Command{
			Name: args[0],
			Args: args[1:],
		}
		iter, err := h.underlying.HandleCommand(ctx, cmd)
		if errors.Is(err, repl.ErrNotFound) {
			return next(ctx, args)
		}
		if err != nil {
			_, _ = fmt.Fprintln(hc.Stderr, err.Error())
			return interp.ExitStatus(1)
		}
		defer func() { _ = iter.Close() }()

		first := true
		for {
			item, ok := iter.Next(ctx)
			if !ok {
				break
			}
			text := responsiveToText(item, pipeWidth)
			if !first {
				_, _ = fmt.Fprintln(hc.Stdout)
			}
			first = false
			_, _ = fmt.Fprint(hc.Stdout, text)
		}
		if err := iter.Err(); err != nil {
			_, _ = fmt.Fprintln(hc.Stderr, err.Error())
			return interp.ExitStatus(1)
		}
		return nil
	}
}

func reconstructLine(cmd repl.Command) string {
	if cmd.Name == "" {
		return ""
	}
	if len(cmd.Args) == 0 {
		return cmd.Name
	}
	return cmd.Name + " " + strings.Join(cmd.Args, " ")
}

func responsiveToText(r component.Responsive, width int) string {
	height := r.Height(width)
	if height <= 0 {
		return ""
	}
	w := term.NewStringWriter(width, height)
	r.Resize(width, height)
	r.Draw(w)
	_ = w.Flush()
	return trimTrailingWhitespace(w.String())
}

func trimTrailingWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	// Remove trailing empty lines.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// pathCompletions scans $PATH directories for executable
// names that start with prefix, deduplicates and returns
// them sorted.
func pathCompletions(prefix string) []string {
	if prefix == "" {
		return nil
	}
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}
	seen := make(map[string]bool)
	for _, dir := range filepath.SplitList(pathEnv) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			if seen[name] {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.Mode()&0111 == 0 {
				continue
			}
			seen[name] = true
		}
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// fileCompletions lists directory entries matching the
// given prefix, appending "/" for directories. Hidden
// files are only included when the prefix starts with ".".
func fileCompletions(prefix string) []string {
	dir, base := filepath.Split(prefix)
	if dir == "" {
		dir = "."
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		// Skip hidden files unless the prefix starts with ".".
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		path := name
		if dir != "." {
			path = filepath.Join(dir, name)
		}
		if e.IsDir() {
			path += "/"
		}
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}
