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

const blockquoteIndent = 2

type blockquoteBlock struct {
	content []textRun
	nested  *blockquoteBlock
	cfg     *Config
	w       int // width from last Height call
}

var _ block = (*blockquoteBlock)(nil)

func newBlockquoteBlock(
	content []textRun, nested *blockquoteBlock, cfg *Config,
) *blockquoteBlock {
	return &blockquoteBlock{
		content: content,
		nested:  nested,
		cfg:     cfg,
	}
}

func (b *blockquoteBlock) Height(width int) int {
	return b.heightAtIndent(width, 0)
}

func (b *blockquoteBlock) Resize(width, _ int) {
	b.w = width
	if b.nested != nil {
		b.nested.Resize(width, 0)
	}
}

func (b *blockquoteBlock) Draw(w term.Writer) {
	contentHeight := b.heightAtIndent(b.w, 0)
	if b.cfg.Blockquote.Bg != tcell.ColorDefault && contentHeight > 1 {
		bgAttr := term.Attributes{Bg: b.cfg.Blockquote.Bg}
		for y := range contentHeight - 1 {
			for x := range b.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}
	b.drawAtIndent(w, 0, 0)
}

func (b *blockquoteBlock) Dimensions() (width, height int) {
	return b.dimensionsAtIndent(0)
}

func (b *blockquoteBlock) SpanAt(x, y int) (text, url string, ok bool) {
	return b.spanAtIndent(x, y, 0)
}

func (b *blockquoteBlock) CharAt(x, y int) (rune, bool) {
	return b.charAtIndent(x, y, 0)
}

func (b *blockquoteBlock) heightAtIndent(width, indent int) int {
	if width <= indent+blockquoteIndent {
		return 0
	}

	h := 0
	effectiveWidth := width - indent - blockquoteIndent

	for _, content := range b.content {
		lines := countWrappedLines(content, effectiveWidth)
		h += lines
	}

	if b.nested != nil {
		h += b.nested.heightAtIndent(width, indent+blockquoteIndent)
	}

	return h + 1
}

func (b *blockquoteBlock) drawAtIndent(w term.Writer, y, indent int) int {
	if b.w <= indent+blockquoteIndent {
		return 0
	}

	startY := y
	effectiveWidth := b.w - indent - blockquoteIndent

	for _, content := range b.content {
		lines := wrapTextRun(content, effectiveWidth)
		for lineIdx, line := range lines {
			w.SetCell(term.Coordinates{X: indent, Y: y + lineIdx}, term.Cell{
				Ch:         b.cfg.BlockquoteBorder,
				Width:      1,
				Attributes: b.cfg.Blockquote,
			})

			x := indent + blockquoteIndent
			for _, sp := range line {
				attr := resolveStyle(sp.style, b.cfg)
				attr = term.AttributesUnion(attr, b.cfg.Blockquote)
				for _, r := range sp.text {
					if x >= b.w {
						break
					}
					w.SetCell(term.Coordinates{X: x, Y: y + lineIdx}, term.Cell{
						Ch:         r,
						Width:      1,
						Attributes: attr,
					})
					x++
				}
			}
		}
		y += len(lines)
		if len(lines) == 0 {
			w.SetCell(term.Coordinates{X: indent, Y: y}, term.Cell{
				Ch:         b.cfg.BlockquoteBorder,
				Width:      1,
				Attributes: b.cfg.Blockquote,
			})
			y++
		}
	}

	if b.nested != nil {
		nestedHeight := b.nested.drawAtIndent(w, y, indent+blockquoteIndent)
		for i := 0; i < nestedHeight-1; i++ {
			w.SetCell(term.Coordinates{X: indent, Y: y + i}, term.Cell{
				Ch:         b.cfg.BlockquoteBorder,
				Width:      1,
				Attributes: b.cfg.Blockquote,
			})
		}
		y += nestedHeight
	}

	return y - startY + 1
}

func (b *blockquoteBlock) dimensionsAtIndent(indent int) (width, height int) {
	maxWidth := 0
	totalHeight := 0

	for _, content := range b.content {
		contentWidth := indent + blockquoteIndent + content.Len()
		if contentWidth > maxWidth {
			maxWidth = contentWidth
		}
		totalHeight++
	}

	if b.nested != nil {
		nestedW, nestedH := b.nested.dimensionsAtIndent(indent + blockquoteIndent)
		if nestedW > maxWidth {
			maxWidth = nestedW
		}
		totalHeight += nestedH - 1
	}

	return maxWidth, totalHeight + 1
}

func (b *blockquoteBlock) spanAtIndent(x, y, indent int) (text, url string, ok bool) {
	if b.w <= indent+blockquoteIndent {
		return
	}

	effectiveWidth := b.w - indent - blockquoteIndent
	currentY := 0

	for _, content := range b.content {
		lines := wrapTextRun(content, effectiveWidth)
		lineCount := len(lines)
		if lineCount == 0 {
			lineCount = 1
		}

		if y >= currentY && y < currentY+lineCount {
			lineIdx := y - currentY
			if lineIdx < len(lines) {
				adjustedX := x - indent - blockquoteIndent
				if adjustedX >= 0 {
					return spanAtInLine(lines[lineIdx], adjustedX)
				}
			}
			return
		}
		currentY += lineCount
	}

	if b.nested != nil {
		nestedHeight := b.nested.heightAtIndent(b.w, indent+blockquoteIndent) - 1
		if y >= currentY && y < currentY+nestedHeight {
			return b.nested.spanAtIndent(x, y-currentY, indent+blockquoteIndent)
		}
	}
	return
}

func (b *blockquoteBlock) charAtIndent(x, y, indent int) (rune, bool) {
	if b.w <= indent+blockquoteIndent {
		return 0, false
	}

	effectiveWidth := b.w - indent - blockquoteIndent
	currentY := 0

	for _, content := range b.content {
		lines := wrapTextRun(content, effectiveWidth)
		lineCount := len(lines)
		if lineCount == 0 {
			lineCount = 1
		}

		if y >= currentY && y < currentY+lineCount {
			lineIdx := y - currentY
			if x == indent {
				return b.cfg.BlockquoteBorder, true
			}
			if lineIdx < len(lines) {
				adjustedX := x - indent - blockquoteIndent
				if adjustedX >= 0 {
					return charAtInLine(lines[lineIdx], adjustedX)
				}
			}
			return 0, false
		}
		currentY += lineCount
	}

	if b.nested != nil {
		nestedHeight := b.nested.heightAtIndent(b.w, indent+blockquoteIndent) - 1
		if y >= currentY && y < currentY+nestedHeight {
			return b.nested.charAtIndent(x, y-currentY, indent+blockquoteIndent)
		}
	}
	return 0, false
}
