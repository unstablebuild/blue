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

func posToCoord(p semanticapi.Position) term.Coordinates {
	return term.Coordinates{X: int(p.Character), Y: int(p.Line)}
}

func coordToPos(c term.Coordinates) semanticapi.Position {
	return semanticapi.Position{Line: uint32(c.Y), Character: uint32(c.X)}
}

func textDocID(uri workspaceapi.URI) semanticapi.TextDocumentIdentifier {
	return semanticapi.TextDocumentIdentifier{
		URI: uriToLSP(uri),
	}
}

func uriToLSP(u workspaceapi.URI) string {
	return fmt.Sprintf("file://%s", u.Path())
}

func lspToURI(s string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI(s)
}

func applyEdits(
	ctx context.Context, ce textapi.CellEditor, edits []semanticapi.TextEdit,
) error {
	sorted := make([]semanticapi.TextEdit, len(edits))
	copy(sorted, edits)

	sort.Slice(sorted, func(i, j int) bool {
		si := sorted[i].Range.Start
		sj := sorted[j].Range.Start
		if si.Line != sj.Line {
			return si.Line > sj.Line
		}
		return si.Character > sj.Character
	})

	for _, edit := range sorted {
		start := posToCoord(edit.Range.Start)
		end := posToCoord(edit.Range.End)
		if _, _, _, err := ce.Edit(
			ctx, start, end, edit.NewText,
		); err != nil {
			return err
		}
	}
	return nil
}
