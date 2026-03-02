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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// bufCellEditor is a CellEditor that maintains an in-memory text buffer.
type bufCellEditor struct {
	lines []string
}

func newBufCellEditor(initial string) *bufCellEditor {
	return &bufCellEditor{lines: strings.Split(initial, "\n")}
}

func (b *bufCellEditor) Edit(
	_ context.Context, start, end term.Coordinates, text string,
) (term.Coordinates, term.Coordinates, string, error) {
	if start.Y >= len(b.lines) {
		start.Y = len(b.lines) - 1
	}
	if start.X > len(b.lines[start.Y]) {
		start.X = len(b.lines[start.Y])
	}
	if end.Y >= len(b.lines) {
		end.Y = len(b.lines) - 1
	}
	if end.X > len(b.lines[end.Y]) {
		end.X = len(b.lines[end.Y])
	}
	before := b.lines[start.Y][:start.X]
	after := b.lines[end.Y][end.X:]
	combined := before + text + after
	newLines := strings.Split(combined, "\n")
	result := make([]string, 0, start.Y+len(newLines)+len(b.lines)-end.Y-1)
	result = append(result, b.lines[:start.Y]...)
	result = append(result, newLines...)
	result = append(result, b.lines[end.Y+1:]...)
	b.lines = result
	return start, end, "", nil
}

func (b *bufCellEditor) String() string {
	return strings.Join(b.lines, "\n")
}

func pos(line, char uint32) semanticapi.Position {
	return semanticapi.Position{Line: line, Character: char}
}

func insertAt(line, char uint32, text string) semanticapi.TextEdit {
	p := pos(line, char)
	return semanticapi.TextEdit{
		Range:   semanticapi.Range{Start: p, End: p},
		NewText: text,
	}
}

func TestApplyEdits_SamePositionInserts(t *testing.T) {
	t.Parallel()

	// Three inserts at (0,0): the array order must define
	// the resulting text order per the LSP spec.
	edits := []semanticapi.TextEdit{
		insertAt(0, 0, "package main\n\n"),
		insertAt(0, 0, "import \"testing\"\n\n"),
		insertAt(0, 0, "func TestFoo(t *testing.T) {}\n"),
	}

	buf := newBufCellEditor("")
	err := ApplyEdits(t.Context(), buf, edits)
	require.NoError(t, err)

	got := buf.String()
	assert.True(t, strings.HasPrefix(got, "package main"),
		"result should start with 'package main', got:\n%s", got)
	assert.Contains(t, got, "import \"testing\"")
	assert.Contains(t, got, "func TestFoo")

	// Verify the ordering: package before import before func.
	pkgIdx := strings.Index(got, "package main")
	impIdx := strings.Index(got, "import \"testing\"")
	funIdx := strings.Index(got, "func TestFoo")
	assert.Less(t, pkgIdx, impIdx, "package should come before import")
	assert.Less(t, impIdx, funIdx, "import should come before func")
}

func TestApplyEdits_MixedPositions(t *testing.T) {
	t.Parallel()

	// Edits at different positions plus same-position inserts.
	buf := newBufCellEditor("line0\nline1\nline2\n")
	edits := []semanticapi.TextEdit{
		// Two inserts at (1,0)
		insertAt(1, 0, "A"),
		insertAt(1, 0, "B"),
		// One edit at (2,0)
		insertAt(2, 0, "C"),
	}

	err := ApplyEdits(t.Context(), buf, edits)
	require.NoError(t, err)

	got := buf.String()
	// At line 1, "A" should appear before "B" per array order.
	lines := strings.Split(got, "\n")
	// line0 is unchanged, line1 should start with AB.
	assert.True(t, strings.HasPrefix(lines[1], "AB"),
		"same-position inserts at line 1 should be AB, got line: %q", lines[1])
}

func TestApplyEdits_SingleEdit(t *testing.T) {
	t.Parallel()

	// Single edit: no ordering concerns.
	buf := newBufCellEditor("")
	edits := []semanticapi.TextEdit{
		insertAt(0, 0, "package main\n"),
	}

	err := ApplyEdits(t.Context(), buf, edits)
	require.NoError(t, err)
	assert.Equal(t, "package main\n", buf.String())
}
