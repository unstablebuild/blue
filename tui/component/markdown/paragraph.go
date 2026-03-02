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

type paragraphBlock struct {
	content textRun
	cfg     *Config
	w       int // width from last Height call
}

var _ block = (*paragraphBlock)(nil)

func newParagraphBlock(content textRun, cfg *Config) *paragraphBlock {
	return &paragraphBlock{content: content, cfg: cfg}
}

func (p *paragraphBlock) Height(width int) int {
	if width <= 0 {
		return 0
	}
	lines := countWrappedLines(p.content, width)
	return lines + 1
}

func (p *paragraphBlock) Resize(width, _ int) {
	p.w = width
}

func (p *paragraphBlock) Draw(w term.Writer) {
	if p.w <= 0 {
		return
	}

	lines := wrapTextRun(p.content, p.w)

	if p.cfg.Paragraph.Bg != tcell.ColorDefault {
		bgAttr := term.Attributes{Bg: p.cfg.Paragraph.Bg}
		for y := range len(lines) {
			for x := range p.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}
	for i, line := range lines {
		x := 0
		for _, sp := range line {
			attr := resolveStyle(sp.style, p.cfg)
			for _, r := range sp.text {
				if x >= p.w {
					break
				}
				w.SetCell(term.Coordinates{X: x, Y: i}, term.Cell{
					Ch:         r,
					Width:      1,
					Attributes: attr,
				})
				x++
			}
		}
	}
}

func (p *paragraphBlock) Dimensions() (width, height int) {
	return p.content.Len(), 2
}

func (p *paragraphBlock) SpanAt(x, y int) (text, url string, ok bool) {
	if p.w <= 0 {
		return
	}
	lines := wrapTextRun(p.content, p.w)
	if y < 0 || y >= len(lines) {
		return
	}
	return spanAtInLine(lines[y], x)
}

func (p *paragraphBlock) CharAt(x, y int) (rune, bool) {
	if p.w <= 0 {
		return 0, false
	}
	lines := wrapTextRun(p.content, p.w)
	if y < 0 || y >= len(lines) {
		return 0, false
	}
	return charAtInLine(lines[y], x)
}
