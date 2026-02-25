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

type headerBlock struct {
	level   int
	content textRun
	cfg     *Config
	w       int // width from last Height call
}

var _ block = (*headerBlock)(nil)

func newHeaderBlock(level int, content textRun, cfg *Config) *headerBlock {
	return &headerBlock{level: level, content: content, cfg: cfg}
}

func (h *headerBlock) Height(width int) int {
	h.w = width
	if width <= 0 {
		return 0
	}
	prefixLen := h.prefixLen()
	effectiveWidth := max(1, width-prefixLen)
	lines := countWrappedLines(h.content, effectiveWidth)
	// 1 (above) + content lines + 1 (standard spacing)
	return lines + 2
}

func (hb *headerBlock) Draw(w term.Writer) {
	if hb.w <= 0 {
		return
	}

	attr := hb.getAttr()
	prefixLen := hb.prefixLen()
	effectiveWidth := max(1, hb.w-prefixLen)
	lines := wrapTextRun(hb.content, effectiveWidth)
	startY := 1

	if attr.Bg != tcell.ColorDefault {
		maxContentWidth := 0
		for i, line := range lines {
			lineWidth := line.Len()
			if i == 0 {
				lineWidth += prefixLen
			} else {
				lineWidth += prefixLen
			}
			if lineWidth > maxContentWidth {
				maxContentWidth = lineWidth
			}
		}

		bgWidth := min(maxContentWidth+2, hb.w)
		bgAttr := term.Attributes{Bg: attr.Bg}
		for lineIdx := range lines {
			y := startY + lineIdx
			for x := range bgWidth {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}

	contentOffset := 0
	if attr.Bg != tcell.ColorDefault {
		contentOffset = 1
	}

	for i, line := range lines {
		y := startY + i
		x := contentOffset

		if i == 0 && hb.cfg.HeaderPrefix {
			for j := 0; j < hb.level; j++ {
				if x >= hb.w {
					break
				}
				w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
					Ch:         '#',
					Width:      1,
					Attributes: attr,
				})
				x++
			}
			if x < hb.w {
				w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
					Ch:         ' ',
					Width:      1,
					Attributes: attr,
				})
				x++
			}
		} else if i > 0 {
			x = contentOffset + prefixLen
		}

		for _, sp := range line {
			for _, r := range sp.text {
				if x >= hb.w {
					break
				}
				w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
					Ch:         r,
					Width:      1,
					Attributes: attr,
				})
				x++
			}
		}
	}
}

func (h *headerBlock) Dimensions() (width, height int) {
	prefixLen := h.prefixLen()
	// 1 (above) + 1 (content line) + 1 (standard spacing)
	return prefixLen + h.content.Len(), 3
}

func (hb *headerBlock) SpanAt(x, y int) (text, url string, ok bool) {
	if hb.w <= 0 {
		return
	}
	if y < 1 {
		return
	}
	y--

	prefixLen := hb.prefixLen()
	effectiveWidth := max(1, hb.w-prefixLen)
	lines := wrapTextRun(hb.content, effectiveWidth)
	if y >= len(lines) {
		return
	}

	attr := hb.getAttr()
	contentOffset := 0
	if attr.Bg != 0 {
		contentOffset = 1
	}

	if y == 0 {
		x -= contentOffset + prefixLen
	} else {
		x -= contentOffset + prefixLen
	}

	if x < 0 {
		return
	}
	return spanAtInLine(lines[y], x)
}

func (hb *headerBlock) CharAt(x, y int) (rune, bool) {
	if hb.w <= 0 {
		return 0, false
	}
	if y < 1 {
		return 0, false
	}
	y--

	prefixLen := hb.prefixLen()
	effectiveWidth := max(1, hb.w-prefixLen)
	lines := wrapTextRun(hb.content, effectiveWidth)
	if y >= len(lines) {
		return 0, false
	}

	attr := hb.getAttr()
	contentOffset := 0
	if attr.Bg != 0 {
		contentOffset = 1
	}

	if y == 0 && x >= contentOffset && x < contentOffset+prefixLen {
		prefixIdx := x - contentOffset
		if prefixIdx < hb.level {
			return '#', true
		}
		return ' ', true
	}

	x -= contentOffset + prefixLen
	if x < 0 {
		return 0, false
	}
	return charAtInLine(lines[y], x)
}

func (h *headerBlock) prefixLen() int {
	if !h.cfg.HeaderPrefix {
		return 0
	}
	return h.level + 1 // "# ", "## ", etc.
}

func (h *headerBlock) getAttr() term.Attributes {
	switch h.level {
	case 1:
		return h.cfg.H1
	case 2:
		return h.cfg.H2
	case 3:
		return h.cfg.H3
	case 4:
		return h.cfg.H4
	case 5:
		return h.cfg.H5
	default:
		return h.cfg.H6
	}
}
