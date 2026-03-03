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

package markdown

import (
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

type tableAlignment int

const (
	alignLeft tableAlignment = iota
	alignCenter
	alignRight
)

type tableBlock struct {
	header     []textRun
	rows       [][]textRun
	alignments []tableAlignment
	cfg        *Config
	w          int // width from last Resize call

	// Cached wrapping state (computed in Resize).
	colWidths     []int
	wrappedHeader [][]textRun  // [col] → wrapped lines
	wrappedRows   [][][]textRun // [row][col] → wrapped lines
	headerHeight  int
	rowHeights    []int
}

var _ block = (*tableBlock)(nil)

func newTableBlock(
	header []textRun,
	rows [][]textRun,
	alignments []tableAlignment,
	cfg *Config,
) *tableBlock {
	return &tableBlock{
		header:     header,
		rows:       rows,
		alignments: alignments,
		cfg:        cfg,
	}
}

func (t *tableBlock) Height(width int) int {
	if width <= 0 || len(t.header) == 0 {
		return 0
	}

	numCols := len(t.header)
	colWidths := t.calculateColumnWidths(width, numCols)

	headerHeight := 1
	for i, h := range t.header {
		if n := countWrappedLines(h, colWidths[i]); n > headerHeight {
			headerHeight = n
		}
	}

	totalRowHeight := 0
	for _, row := range t.rows {
		rowHeight := 1
		for i := range numCols {
			var cell textRun
			if i < len(row) {
				cell = row[i]
			}
			if n := countWrappedLines(cell, colWidths[i]); n > rowHeight {
				rowHeight = n
			}
		}
		totalRowHeight += rowHeight
	}

	// top border + header + separator + data rows + bottom border + spacing
	return 1 + headerHeight + 1 + totalRowHeight + 1 + 1
}

func (t *tableBlock) Resize(width, _ int) {
	t.w = width
	if width <= 0 || len(t.header) == 0 {
		return
	}

	numCols := len(t.header)
	t.colWidths = t.calculateColumnWidths(width, numCols)

	// Wrap header cells.
	t.wrappedHeader = make([][]textRun, numCols)
	t.headerHeight = 1
	for i, h := range t.header {
		t.wrappedHeader[i] = wrapTextRun(h, t.colWidths[i])
		// wrapTextRun returns nil for empty runs; normalise to one
		// empty line so every cell has at least one entry, matching
		// countWrappedLines which returns 1 for the same input.
		if len(t.wrappedHeader[i]) == 0 {
			t.wrappedHeader[i] = []textRun{nil}
		}
		if len(t.wrappedHeader[i]) > t.headerHeight {
			t.headerHeight = len(t.wrappedHeader[i])
		}
	}

	// Wrap data row cells.
	t.wrappedRows = make([][][]textRun, len(t.rows))
	t.rowHeights = make([]int, len(t.rows))
	for r, row := range t.rows {
		t.wrappedRows[r] = make([][]textRun, numCols)
		t.rowHeights[r] = 1
		for i := range numCols {
			var cell textRun
			if i < len(row) {
				cell = row[i]
			}
			t.wrappedRows[r][i] = wrapTextRun(cell, t.colWidths[i])
			// See header normalisation comment above.
			if len(t.wrappedRows[r][i]) == 0 {
				t.wrappedRows[r][i] = []textRun{nil}
			}
			if len(t.wrappedRows[r][i]) > t.rowHeights[r] {
				t.rowHeights[r] = len(t.wrappedRows[r][i])
			}
		}
	}
}

func (t *tableBlock) cachedHeight() int {
	total := 0
	for _, rh := range t.rowHeights {
		total += rh
	}
	// top border + header + separator + data rows + bottom border + spacing
	return 1 + t.headerHeight + 1 + total + 1 + 1
}

func (t *tableBlock) Draw(w term.Writer) {
	if t.w <= 0 || len(t.header) == 0 {
		return
	}

	contentHeight := t.cachedHeight()
	if t.cfg.Paragraph.Bg != tcell.ColorDefault && contentHeight > 1 {
		bgAttr := term.Attributes{Bg: t.cfg.Paragraph.Bg}
		for y := range contentHeight - 1 {
			for x := range t.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}

	cs := t.cfg.TableCharSet

	y := 0
	y = t.renderHorizontalBorder(w, y, cs.TopLeft, cs.HorizontalTop, cs.TopJoin, cs.TopRight)
	y = t.renderRow(w, y, t.wrappedHeader, t.headerHeight, cs, true)
	y = t.renderHorizontalBorder(w, y, cs.Left, cs.HeaderSeparator, cs.CrossJoin, cs.Right)
	for r := range t.rows {
		y = t.renderRow(w, y, t.wrappedRows[r], t.rowHeights[r], cs, false)
	}
	t.renderHorizontalBorder(w, y, cs.BottomLeft, cs.HorizontalBottom, cs.BottomJoin, cs.BottomRight)
}

func (t *tableBlock) Dimensions() (width, height int) {
	if len(t.header) == 0 {
		return 0, 0
	}

	numCols := len(t.header)
	colWidths := make([]int, numCols)

	for i, h := range t.header {
		if h.Len() > colWidths[i] {
			colWidths[i] = h.Len()
		}
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if i < numCols && cell.Len() > colWidths[i] {
				colWidths[i] = cell.Len()
			}
		}
	}

	totalWidth := 1 // left border
	for _, w := range colWidths {
		totalWidth += w + 1 // column + separator
	}

	// top + header + separator + rows + bottom + spacing
	totalHeight := 1 + 1 + 1 + len(t.rows) + 1 + 1

	return totalWidth, totalHeight
}

func (t *tableBlock) SpanAt(x, y int) (text, url string, ok bool) {
	if t.w <= 0 || len(t.header) == 0 {
		return
	}

	// Top border.
	if y == 0 {
		return
	}

	// Header lines.
	sepY := 1 + t.headerHeight
	if y >= 1 && y < sepY {
		return t.spanAtInWrappedRow(x, t.wrappedHeader, y-1)
	}

	// Separator.
	if y == sepY {
		return
	}

	// Data rows. Bottom border and spacing fall through
	// without matching any row, returning empty.
	dataY := y - (sepY + 1)
	cumY := 0
	for r, rh := range t.rowHeights {
		if dataY < cumY+rh {
			return t.spanAtInWrappedRow(x, t.wrappedRows[r], dataY-cumY)
		}
		cumY += rh
	}
	return
}

func (t *tableBlock) spanAtInWrappedRow(
	x int, wrappedCells [][]textRun, lineInRow int,
) (text, url string, ok bool) {
	colX := 1
	for i, colWidth := range t.colWidths {
		if x >= colX && x < colX+colWidth {
			if i < len(wrappedCells) && lineInRow < len(wrappedCells[i]) {
				return spanAtInLine(wrappedCells[i][lineInRow], x-colX)
			}
			return
		}
		colX += colWidth + 1
	}
	return
}

func (t *tableBlock) CharAt(x, y int) (rune, bool) {
	if t.w <= 0 || len(t.header) == 0 {
		return 0, false
	}

	cs := t.cfg.TableCharSet
	sepY := 1 + t.headerHeight
	totalDataHeight := 0
	for _, rh := range t.rowHeights {
		totalDataHeight += rh
	}
	bottomY := sepY + 1 + totalDataHeight

	// Top border.
	if y == 0 {
		return t.charAtHorizontalBorder(
			x, cs.TopLeft, cs.HorizontalTop, cs.TopJoin, cs.TopRight,
		)
	}

	// Separator.
	if y == sepY {
		return t.charAtHorizontalBorder(
			x, cs.Left, cs.HeaderSeparator, cs.CrossJoin, cs.Right,
		)
	}

	// Bottom border.
	if y == bottomY {
		return t.charAtHorizontalBorder(
			x, cs.BottomLeft, cs.HorizontalBottom, cs.BottomJoin, cs.BottomRight,
		)
	}

	// Content rows.
	var wrappedCells [][]textRun
	var lineInRow int

	if y >= 1 && y < sepY {
		wrappedCells = t.wrappedHeader
		lineInRow = y - 1
	} else if y > sepY && y < bottomY {
		dataY := y - (sepY + 1)
		cumY := 0
		for r, rh := range t.rowHeights {
			if dataY < cumY+rh {
				wrappedCells = t.wrappedRows[r]
				lineInRow = dataY - cumY
				break
			}
			cumY += rh
		}
	} else {
		return 0, false
	}

	if x == 0 {
		return cs.ColumnSeparator, true
	}

	colX := 1
	for i, colWidth := range t.colWidths {
		if x >= colX && x < colX+colWidth {
			if i < len(wrappedCells) && lineInRow < len(wrappedCells[i]) {
				ch, ok := charAtInLine(wrappedCells[i][lineInRow], x-colX)
				if ok {
					return ch, true
				}
			}
			return ' ', true
		}
		colX += colWidth
		if x == colX {
			return cs.ColumnSeparator, true
		}
		colX++
	}
	return 0, false
}

func (t *tableBlock) charAtHorizontalBorder(
	x int, left, fill, join, right rune,
) (rune, bool) {
	if x == 0 {
		return left, true
	}
	colX := 1
	for i, colWidth := range t.colWidths {
		if x >= colX && x < colX+colWidth {
			return fill, true
		}
		colX += colWidth
		if x == colX {
			if i == len(t.colWidths)-1 {
				return right, true
			}
			return join, true
		}
		colX++
	}
	return 0, false
}

func (t *tableBlock) calculateColumnWidths(
	totalWidth, numCols int,
) []int {
	if numCols == 0 {
		return nil
	}
	availableWidth := totalWidth - (numCols + 1)
	baseWidth := availableWidth / numCols
	remainder := availableWidth % numCols

	widths := make([]int, numCols)
	for i := range widths {
		widths[i] = baseWidth
		if i < remainder {
			widths[i]++
		}
		if widths[i] < 1 {
			widths[i] = 1
		}
	}
	return widths
}

func (t *tableBlock) renderHorizontalBorder(
	w term.Writer, y int, left, fill, join, right rune,
) int {
	x := 0
	w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
		Ch:         left,
		Width:      1,
		Attributes: t.cfg.Paragraph,
	})
	x++

	for i, colWidth := range t.colWidths {
		for j := range colWidth {
			w.SetCell(term.Coordinates{X: x + j, Y: y}, term.Cell{
				Ch:         fill,
				Width:      1,
				Attributes: t.cfg.Paragraph,
			})
		}
		x += colWidth

		ch := join
		if i == len(t.colWidths)-1 {
			ch = right
		}
		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
			Ch:         ch,
			Width:      1,
			Attributes: t.cfg.Paragraph,
		})
		x++
	}
	return y + 1
}

func (t *tableBlock) renderRow(
	w term.Writer, y int, wrappedCells [][]textRun,
	rowHeight int, cs TableCharSet, isHeader bool,
) int {
	for line := range rowHeight {
		x := 0
		w.SetCell(term.Coordinates{X: x, Y: y + line}, term.Cell{
			Ch:         cs.ColumnSeparator,
			Width:      1,
			Attributes: t.cfg.Paragraph,
		})
		x++

		for i, colWidth := range t.colWidths {
			var lineRun textRun
			if i < len(wrappedCells) && line < len(wrappedCells[i]) {
				lineRun = wrappedCells[i][line]
			}
			t.renderCell(w, x, y+line, colWidth, lineRun, t.getAlignment(i), isHeader)
			x += colWidth

			w.SetCell(term.Coordinates{X: x, Y: y + line}, term.Cell{
				Ch:         cs.ColumnSeparator,
				Width:      1,
				Attributes: t.cfg.Paragraph,
			})
			x++
		}
	}
	return y + rowHeight
}

func (t *tableBlock) renderCell(
	w term.Writer, x, y, width int, content textRun,
	align tableAlignment, isHeader bool,
) {
	text := content.String()
	textLen := len(text)
	if textLen > width {
		text = text[:width]
		textLen = width
	}

	var padding int
	switch align {
	case alignCenter:
		padding = (width - textLen) / 2
	case alignRight:
		padding = width - textLen
	}

	attr := t.cfg.Paragraph
	if isHeader {
		attr = t.cfg.Bold
	}

	spIdx := 0
	runeIdx := 0
	for i, r := range text {
		cellX := x + padding + i
		cellAttr := attr
		if spIdx < len(content) {
			sp := content[spIdx]
			if !isHeader {
				cellAttr = resolveStyle(sp.style, t.cfg)
			}
			runeIdx++
			if runeIdx >= len(sp.text) {
				spIdx++
				runeIdx = 0
			}
		}
		w.SetCell(term.Coordinates{X: cellX, Y: y}, term.Cell{
			Ch:         r,
			Width:      1,
			Attributes: cellAttr,
		})
	}
}

func (t *tableBlock) getAlignment(col int) tableAlignment {
	if col < len(t.alignments) {
		return t.alignments[col]
	}
	return alignLeft
}
