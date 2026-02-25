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
	w          int // width from last Height call
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
	t.w = width
	if width <= 0 || len(t.header) == 0 {
		return 0
	}
	// top border + header + separator + rows + bottom border + spacing
	return 1 + 1 + 1 + len(t.rows) + 1 + 1
}

func (t *tableBlock) Draw(w term.Writer) {
	if t.w <= 0 || len(t.header) == 0 {
		return
	}

	contentHeight := t.Height(t.w)
	if t.cfg.Paragraph.Bg != tcell.ColorDefault && contentHeight > 1 {
		bgAttr := term.Attributes{Bg: t.cfg.Paragraph.Bg}
		for y := range contentHeight - 1 {
			for x := range t.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}

	numCols := len(t.header)
	colWidths := t.calculateColumnWidths(t.w, numCols)
	cs := t.cfg.TableCharSet

	y := 0
	y = t.renderTopBorder(w, y, colWidths, cs)
	y = t.renderHeaderRow(w, y, colWidths, cs)
	y = t.renderSeparator(w, y, colWidths, cs)
	for _, row := range t.rows {
		y = t.renderDataRow(w, y, row, colWidths, cs)
	}
	t.renderBottomBorder(w, y, colWidths, cs)
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

// SpanAt returns the text and URL at the given position.
// y=0: top border, y=1: header, y=2: separator, y=3+: data rows
func (t *tableBlock) SpanAt(x, y int) (text, url string, ok bool) {
	if t.w <= 0 || len(t.header) == 0 {
		return
	}

	numCols := len(t.header)
	colWidths := t.calculateColumnWidths(t.w, numCols)

	if y == 0 || y == 2 {
		return
	}

	var row []textRun
	if y == 1 {
		row = t.header
	} else if y >= 3 && y < 3+len(t.rows) {
		row = t.rows[y-3]
	} else {
		return
	}

	colX := 1 // start after left border
	for i, colWidth := range colWidths {
		if x >= colX && x < colX+colWidth {
			if i < len(row) {
				text := row[i].String()
				if len(text) > colWidth {
					text = text[:colWidth]
				}
				return text, "", len(text) > 0
			}
			return
		}
		colX += colWidth + 1 // +1 for column separator
	}
	return
}

func (t *tableBlock) CharAt(x, y int) (rune, bool) {
	if t.w <= 0 || len(t.header) == 0 {
		return 0, false
	}

	numCols := len(t.header)
	colWidths := t.calculateColumnWidths(t.w, numCols)
	cs := t.cfg.TableCharSet

	if y == 0 {
		if x == 0 {
			return cs.TopLeft, true
		}
		colX := 1
		for i, colWidth := range colWidths {
			if x >= colX && x < colX+colWidth {
				return cs.HorizontalTop, true
			}
			colX += colWidth
			if x == colX {
				if i == len(colWidths)-1 {
					return cs.TopRight, true
				}
				return cs.TopJoin, true
			}
			colX++
		}
		return 0, false
	}

	if y == 2 {
		if x == 0 {
			return cs.Left, true
		}
		colX := 1
		for i, colWidth := range colWidths {
			if x >= colX && x < colX+colWidth {
				return cs.HeaderSeparator, true
			}
			colX += colWidth
			if x == colX {
				if i == len(colWidths)-1 {
					return cs.Right, true
				}
				return cs.CrossJoin, true
			}
			colX++
		}
		return 0, false
	}

	totalHeight := t.Height(t.w) - 1 // exclude spacing
	if y == totalHeight-1 {
		if x == 0 {
			return cs.BottomLeft, true
		}
		colX := 1
		for i, colWidth := range colWidths {
			if x >= colX && x < colX+colWidth {
				return cs.HorizontalBottom, true
			}
			colX += colWidth
			if x == colX {
				if i == len(colWidths)-1 {
					return cs.BottomRight, true
				}
				return cs.BottomJoin, true
			}
			colX++
		}
		return 0, false
	}

	var row []textRun
	if y == 1 {
		row = t.header
	} else if y >= 3 && y < 3+len(t.rows) {
		row = t.rows[y-3]
	} else {
		return 0, false
	}

	if x == 0 {
		return cs.ColumnSeparator, true
	}

	colX := 1
	for i, colWidth := range colWidths {
		if x >= colX && x < colX+colWidth {
			if i < len(row) {
				text := row[i].String()
				cellX := x - colX
				if cellX < len(text) {
					return rune(text[cellX]), true
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

func (t *tableBlock) renderTopBorder(
	w term.Writer, y int, colWidths []int, cs TableCharSet,
) int {
	x := 0
	w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
		Ch:         cs.TopLeft,
		Width:      1,
		Attributes: t.cfg.Paragraph,
	})
	x++

	for i, colWidth := range colWidths {
		for j := range colWidth {
			w.SetCell(term.Coordinates{X: x + j, Y: y}, term.Cell{
				Ch:         cs.HorizontalTop,
				Width:      1,
				Attributes: t.cfg.Paragraph,
			})
		}
		x += colWidth

		ch := cs.TopJoin
		if i == len(colWidths)-1 {
			ch = cs.TopRight
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

func (t *tableBlock) renderHeaderRow(
	w term.Writer, y int, colWidths []int, cs TableCharSet,
) int {
	x := 0
	w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
		Ch:         cs.ColumnSeparator,
		Width:      1,
		Attributes: t.cfg.Paragraph,
	})
	x++

	for i, cell := range t.header {
		if i >= len(colWidths) {
			break
		}
		colWidth := colWidths[i]
		t.renderCell(w, x, y, colWidth, cell, t.getAlignment(i), true)
		x += colWidth

		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
			Ch:         cs.ColumnSeparator,
			Width:      1,
			Attributes: t.cfg.Paragraph,
		})
		x++
	}
	return y + 1
}

func (t *tableBlock) renderSeparator(
	w term.Writer, y int, colWidths []int, cs TableCharSet,
) int {
	x := 0

	w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
		Ch:         cs.Left,
		Width:      1,
		Attributes: t.cfg.Paragraph,
	})
	x++

	for i, colWidth := range colWidths {
		for j := range colWidth {
			w.SetCell(term.Coordinates{X: x + j, Y: y}, term.Cell{
				Ch:         cs.HeaderSeparator,
				Width:      1,
				Attributes: t.cfg.Paragraph,
			})
		}
		x += colWidth

		ch := cs.CrossJoin
		if i == len(colWidths)-1 {
			ch = cs.Right
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

func (t *tableBlock) renderBottomBorder(
	w term.Writer, y int, colWidths []int, cs TableCharSet,
) int {
	x := 0
	w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
		Ch:         cs.BottomLeft,
		Width:      1,
		Attributes: t.cfg.Paragraph,
	})
	x++

	for i, colWidth := range colWidths {
		for j := range colWidth {
			w.SetCell(term.Coordinates{X: x + j, Y: y}, term.Cell{
				Ch:         cs.HorizontalBottom,
				Width:      1,
				Attributes: t.cfg.Paragraph,
			})
		}
		x += colWidth

		ch := cs.BottomJoin
		if i == len(colWidths)-1 {
			ch = cs.BottomRight
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

func (t *tableBlock) renderDataRow(
	w term.Writer, y int, row []textRun,
	colWidths []int, cs TableCharSet,
) int {
	x := 0
	w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
		Ch:         cs.ColumnSeparator,
		Width:      1,
		Attributes: t.cfg.Paragraph,
	})
	x++

	for i, colWidth := range colWidths {
		var cell textRun
		if i < len(row) {
			cell = row[i]
		}
		t.renderCell(w, x, y, colWidth, cell, t.getAlignment(i), false)
		x += colWidth

		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
			Ch:         cs.ColumnSeparator,
			Width:      1,
			Attributes: t.cfg.Paragraph,
		})
		x++
	}
	return y + 1
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
