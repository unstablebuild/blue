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

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// ResolveSymbol resolves a symbol name to a URI and position by querying
// the workspace symbol provider. It prefers an exact name match and falls
// back to the first result (best fuzzy match from the server).
func ResolveSymbol(
	ctx context.Context, lsp semanticapi.LSP, name string,
) (uri string, pos semanticapi.Position, err error) {
	syms, err := lsp.WorkspaceSymbol(ctx, semanticapi.WorkspaceSymbolParams{
		Query: name,
	})
	if err != nil {
		return "", semanticapi.Position{}, err
	}
	if len(syms) == 0 {
		return "", semanticapi.Position{}, fmt.Errorf("no symbols found for %q", name)
	}
	// Prefer an exact name match.
	for _, s := range syms {
		if s.Name == name {
			return s.Location.URI, s.Location.Range.Start, nil
		}
	}
	// Fall back to the first result (best fuzzy match).
	return syms[0].Location.URI, syms[0].Location.Range.Start, nil
}

// CompleteSymbol returns symbol-name completions from the workspace symbol
// provider. It only completes the first argument; if args already contains
// a value, it returns an empty iterator.
func CompleteSymbol(
	ctx context.Context, lsp semanticapi.LSP, args []string,
) (iterator.Iterator[string], error) {
	if len(args) > 0 {
		return iterator.Empty[string](), nil
	}
	syms, err := lsp.WorkspaceSymbol(ctx, semanticapi.WorkspaceSymbolParams{})
	if err != nil {
		return nil, err
	}
	names := make([]string, len(syms))
	for i, s := range syms {
		names[i] = s.Name
	}
	return iterator.FromSlice(names), nil
}
