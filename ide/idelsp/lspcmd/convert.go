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
	"fmt"
	"sort"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// PosToCoord converts an LSP Position to terminal Coordinates.
func PosToCoord(p semanticapi.Position) term.Coordinates {
	return term.Coordinates{X: int(p.Character), Y: int(p.Line)}
}

// CoordToPos converts terminal Coordinates to an LSP Position.
func CoordToPos(c term.Coordinates) semanticapi.Position {
	return semanticapi.Position{Line: uint32(c.Y), Character: uint32(c.X)}
}

// TextDocID converts a workspace URI to an LSP TextDocumentIdentifier.
func TextDocID(uri workspaceapi.URI) semanticapi.TextDocumentIdentifier {
	return semanticapi.TextDocumentIdentifier{
		URI: URIToLSP(uri),
	}
}

// URIToLSP converts a workspace URI to an LSP-compatible file:// URI string.
func URIToLSP(u workspaceapi.URI) string {
	return fmt.Sprintf("file://%s", u.Path())
}

// LspToURI converts an LSP file:// URI string to a workspace URI.
func LspToURI(s string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI(s)
}

// ApplyEdits applies a set of LSP TextEdits to a CellEditor in reverse
// document order so that earlier positions remain valid.
//
// Per the LSP spec, when multiple inserts share the same position,
// the array order defines the order in which the inserted strings
// appear in the resulting text. Since we apply edits sequentially
// from bottom to top, same-position inserts must be reversed so the
// first-in-array insert is applied last (ending up first in the text).
func ApplyEdits(
	ctx context.Context, ce textapi.CellEditor, edits []semanticapi.TextEdit,
) error {
	sorted := make([]semanticapi.TextEdit, len(edits))
	copy(sorted, edits)

	sort.SliceStable(sorted, func(i, j int) bool {
		si := sorted[i].Range.Start
		sj := sorted[j].Range.Start
		if si.Line != sj.Line {
			return si.Line > sj.Line
		}
		return si.Character > sj.Character
	})
	reverseSameStartEdits(sorted)

	for _, edit := range sorted {
		start := PosToCoord(edit.Range.Start)
		end := PosToCoord(edit.Range.End)
		if _, _, _, err := ce.Edit(
			ctx, start, end, edit.NewText,
		); err != nil {
			return err
		}
	}
	return nil
}

// reverseSameStartEdits reverses each contiguous group of edits
// sharing the same start position. This is needed because when
// applying edits bottom-to-top, each insert at position P pushes
// previous text at P downward. Reversing ensures the first-in-array
// insert ends up first in the resulting text, per the LSP spec.
func reverseSameStartEdits(edits []semanticapi.TextEdit) {
	for i := 0; i < len(edits); {
		j := i + 1
		for j < len(edits) &&
			edits[j].Range.Start == edits[i].Range.Start {
			j++
		}
		if j-i > 1 {
			for l, r := i, j-1; l < r; l, r = l+1, r-1 {
				edits[l], edits[r] = edits[r], edits[l]
			}
		}
		i = j
	}
}
