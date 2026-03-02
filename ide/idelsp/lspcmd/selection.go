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
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

var _ textapi.EventHandler = (*SelectionTracker)(nil)

// SelectionTracker tracks active text selections by subscribing to
// EventTypeSelection and EventTypeCursor events.
type SelectionTracker struct {
	mu   sync.Mutex
	sels map[string]semanticapi.Range
}

// NewSelectionTracker creates a new SelectionTracker.
func NewSelectionTracker() *SelectionTracker {
	return &SelectionTracker{
		sels: make(
			map[string]semanticapi.Range,
		),
	}
}

func (s *SelectionTracker) Handle(
	_ context.Context, ev textapi.Event,
) bool {
	uri := URIToLSP(ev.URI)

	s.mu.Lock()
	defer s.mu.Unlock()

	switch ev.Type {
	case textapi.EventTypeSelection:
		start, end := CoordToPos(ev.Start), CoordToPos(ev.End)
		// LSP requires Range.Start <= Range.End; the editor may
		// report a backward selection (user dragged upward).
		if start.Line > end.Line ||
			(start.Line == end.Line && start.Character > end.Character) {
			start, end = end, start
		}
		s.sels[uri] = semanticapi.Range{Start: start, End: end}
	case textapi.EventTypeCursor:
		delete(s.sels, uri)
	}
	return false
}

// Get returns the tracked selection range for the given URI.
func (s *SelectionTracker) Get(
	uri workspaceapi.URI,
) (semanticapi.Range, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.sels[URIToLSP(uri)]
	return r, ok
}

// ClampRange clamps Start and End character offsets so they do not
// exceed the actual line lengths in the document obtained from the
// editor's CellView. If the CellView is nil or RawCells fails,
// the range is returned unchanged.
func ClampRange(
	rng semanticapi.Range, editor textapi.Editor, resource textapi.Handler,
) semanticapi.Range {
	cv := editor.CellView(resource)
	if cv == nil {
		return rng
	}
	cells, err := cv.RawCells()
	if err != nil || len(cells) == 0 {
		return rng
	}

	clamp := func(p semanticapi.Position) semanticapi.Position {
		line := int(p.Line)
		if line >= len(cells) {
			return p
		}
		if lineLen := uint32(len(cells[line])); p.Character > lineLen {
			p.Character = lineLen
		}
		return p
	}
	rng.Start = clamp(rng.Start)
	rng.End = clamp(rng.End)
	return rng
}
