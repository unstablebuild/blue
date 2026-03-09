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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"

	"github.com/unstablebuild/blue/ide/idelsp/lspcmd"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/debug"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func testHandler(
	lsp semanticapi.LSP,
	notify browserapi.Notifications,
	parser syntaxapi.Parser,
	executor workspaceapi.Executor,
) textapi.CommandHandler {
	return &testCmd{
		lsp:      lsp,
		notify:   notify,
		parser:   parser,
		executor: executor,
	}
}

var _ textapi.CommandHandler = (*testCmd)(nil)

type testCmd struct {
	lsp      semanticapi.LSP
	notify   browserapi.Notifications
	parser   syntaxapi.Parser
	executor workspaceapi.Executor
}

type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

type codeLensTestArgs struct {
	URI        string   `json:"URI"`
	Tests      []string `json:"Tests"`
	Benchmarks []string `json:"Benchmarks"`
}

func (h *testCmd) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	if cmd.Resource == nil {
		return nil
	}

	var testName, pkgDir string
	var isBenchmark bool

	if len(cmd.Args) > 0 {
		testName = cmd.Args[0]
		pkgDir = filepath.Dir(cmd.URI.Path())
		isBenchmark = strings.HasPrefix(testName, "Benchmark")
	} else {
		params := semanticapi.CodeLensParams{
			TextDocument: lspcmd.TextDocID(cmd.URI),
		}
		lenses, err := h.lsp.CodeLens(ctx, params)
		if err != nil {
			return fmt.Errorf("code lens: %w", err)
		}

		cursorLine := uint32(cmd.Cursor.Content.Y)
		var nearest *semanticapi.CodeLens
		minDist := uint32(math.MaxUint32)
		for i := range lenses {
			lens := &lenses[i]
			if lens.Command == nil || lens.Command.Command != "gopls.run_tests" {
				continue
			}
			dist := absDiff(lens.Range.Start.Line, cursorLine)
			if dist < minDist {
				minDist = dist
				nearest = lens
			}
		}

		if nearest == nil || nearest.Command == nil {
			_, _ = h.notify.Notify(browserapi.LevelInfo, "No test found near cursor")
			return nil
		}

		if len(nearest.Command.Arguments) == 0 {
			_, _ = h.notify.Notify(browserapi.LevelInfo, "No test found near cursor")
			return nil
		}

		var args codeLensTestArgs
		if err := json.Unmarshal(nearest.Command.Arguments[0], &args); err != nil {
			return fmt.Errorf("parse code lens args: %w", err)
		}

		if len(args.Tests) > 0 {
			testName = args.Tests[0]
		} else if len(args.Benchmarks) > 0 {
			testName = args.Benchmarks[0]
			isBenchmark = true
		} else {
			_, _ = h.notify.Notify(browserapi.LevelInfo, "No test found near cursor")
			return nil
		}

		lensURI := args.URI
		if path, ok := strings.CutPrefix(lensURI, "file://"); ok {
			pkgDir = filepath.Dir(path)
		} else {
			pkgDir = filepath.Dir(cmd.URI.Path())
		}
	}

	var goArgs []string
	if isBenchmark {
		goArgs = []string{"test", "-json", "-count=1", "-run", "^$", "-bench", "^" + testName + "$", "."}
	} else {
		goArgs = []string{"test", "-json", "-count=1", "-run", "^" + testName + "$", "."}
	}

	pr, pw := io.Pipe()
	var stderr bytes.Buffer
	doneCh := make(chan error, 1)
	watcher := workspaceapi.ChanProcessWatcher(doneCh)

	goCmd := workspaceapi.Cmd{
		Path:    "go",
		Args:    goArgs,
		Dir:     pkgDir,
		Stdout:  pw,
		Stderr:  &stderr,
		Watcher: watcher,
	}

	_, err := h.executor.Start(ctx, goCmd)
	if err != nil {
		_ = pw.Close()
		_, _ = h.notify.Notify(browserapi.LevelError, "start go test: %s", err)
		return nil
	}

	// Close pw when the process exits so the scanner on pr gets EOF.
	exitErrCh := make(chan error, 1)
	go func() {
		exitErr := <-doneCh
		_ = pw.Close()
		exitErrCh <- exitErr
	}()

	go debug.CapturePanicReport(func() {
		h.processTestOutput(testName, pr, &stderr, exitErrCh)
	})

	return nil
}

func (h *testCmd) processTestOutput(
	testName string,
	pr *io.PipeReader,
	stderr *bytes.Buffer,
	exitErrCh <-chan error,
) {
	id, _ := h.notify.Notify(browserapi.LevelInfo, "Running %s", testName)

	var completed int64
	var failed bool
	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		var ev testEvent
		if json.Unmarshal(scanner.Bytes(), &ev) != nil {
			continue
		}
		if ev.Test == "" {
			continue
		}
		switch ev.Action {
		case "run":
			msg := fmt.Sprintf("Running %s\n(%d tests done)", ev.Test, completed)
			_ = h.notify.UpdateNotificationProgress(id, msg, completed, completed+1)
		case "pass", "fail", "skip":
			completed++
			if ev.Action == "fail" {
				failed = true
			}
			_ = h.notify.UpdateNotificationProgress(id, "", completed, completed+1)
		}
	}

	exitErr := <-exitErrCh

	// Complete the progress notification.
	if completed > 0 {
		_ = h.notify.UpdateNotificationProgress(id, "", completed, completed)
	} else {
		_ = h.notify.UpdateNotificationProgress(id, "", 1, 1)
	}

	if failed || exitErr != nil {
		msg := fmt.Sprintf("%s failed", testName)
		if completed == 0 && stderr.Len() > 0 {
			msg = strings.TrimSpace(stderr.String())
		}
		_, _ = h.notify.Notify(browserapi.LevelError, "%s", msg)
	} else {
		_, _ = h.notify.Notify(browserapi.LevelSuccess, "%s passed", testName)
	}
}

func (h *testCmd) Complete(
	ctx context.Context, _ string, _ []string,
) (iterator.Iterator[string], error) {
	return completeTestFuncs(ctx, h.parser)
}

// completeTestFuncs returns the names of all Test, Benchmark, Fuzz,
// and Example functions in the workspace by querying the tree-sitter
// parser for Go function declarations.
func completeTestFuncs(
	ctx context.Context, parser syntaxapi.Parser,
) (iterator.Iterator[string], error) {
	iter, err := parser.Search(
		`(function_declaration name: (identifier) @name)`,
		[]string{"name"}, "go",
	)
	if err != nil {
		return nil, err
	}
	return iterator.Filter(
		iterator.Map(iter, func(r syntaxapi.Result) string {
			return r.Text
		}),
		isTestFunc,
	), nil
}

func isTestFunc(name string) bool {
	return strings.HasPrefix(name, "Test") ||
		strings.HasPrefix(name, "Benchmark") ||
		strings.HasPrefix(name, "Fuzz") ||
		strings.HasPrefix(name, "Example")
}
