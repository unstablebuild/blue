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

package walkdir

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// localFS implements Reader using OS calls for testing.
type localFS struct {
	root string
}

func (f localFS) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(f.root, path)
}

func (f localFS) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + f.resolve(path))
}

func (f localFS) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(f.resolve(path), flag, mode)
}

func (f localFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(f.resolve(path))
}

func (f localFS) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(f.resolve(name))
}

func TestReadLines_close_does_not_hang(t *testing.T) {
	dir := t.TempDir()

	// Create enough files with many lines so that ReadLines workers
	// are actively sending when we call Close.
	for i := range 20 {
		subdir := filepath.Join(dir, fmt.Sprintf("pkg%d", i))
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatal(err)
		}
		for j := range 5 {
			var content strings.Builder
			for k := range 50 {
				fmt.Fprintf(&content, "line %d content\n", k)
			}
			if err := os.WriteFile(
				filepath.Join(subdir, fmt.Sprintf("file%d.txt", j)),
				[]byte(content.String()), 0o644,
			); err != nil {
				t.Fatal(err)
			}
		}
	}

	fs := localFS{root: dir}
	ctx := context.Background()

	paths, err := ListFiles(ctx, fs, dir)
	if err != nil {
		t.Fatal(err)
	}

	lines, err := ReadLines(ctx, fs, paths)
	if err != nil {
		t.Fatal(err)
	}

	// Read a handful of lines, then stop early.
	for range 10 {
		_, ok := lines.Next(ctx)
		if !ok {
			t.Fatal("expected more lines")
		}
	}

	// Close must return promptly. Before the fix, this would deadlock
	// because readFile workers were stuck on unbuffered channel sends
	// with no context check.
	done := make(chan struct{})
	go func() {
		_ = lines.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() hung — likely deadlock in readFile workers")
	}
}
