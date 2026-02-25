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
	"strings"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

type codeBlock struct {
	language string
	code     string
	cfg      *Config
	w        int // width from last Height call
}

var _ block = (*codeBlock)(nil)

func newCodeBlock(language, code string, cfg *Config) *codeBlock {
	return &codeBlock{language: language, code: code, cfg: cfg}
}

func (c *codeBlock) Height(width int) int {
	c.w = width
	if width <= 0 {
		return 0
	}
	lines := c.codeLines()
	return len(lines) + 1
}

func (c *codeBlock) Draw(w term.Writer) {
	if c.w <= 0 {
		return
	}

	lines := c.codeLines()

	if c.cfg.CodeBlock.Bg != tcell.ColorDefault {
		bgAttr := term.Attributes{Bg: c.cfg.CodeBlock.Bg}
		for y := range len(lines) {
			for x := range c.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}

	for lineIdx, line := range lines {
		x := 0
		for _, r := range line {
			if x >= c.w {
				break
			}
			w.SetCell(term.Coordinates{X: x, Y: lineIdx}, term.Cell{
				Ch:         r,
				Width:      1,
				Attributes: c.cfg.CodeBlock,
			})
			x++
		}
	}
}

func (c *codeBlock) Dimensions() (width, height int) {
	lines := c.codeLines()
	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}
	return maxWidth, len(lines) + 1
}

func (c *codeBlock) SpanAt(x, y int) (text, url string, ok bool) {
	if c.w <= 0 {
		return
	}
	lines := c.codeLines()
	if y < 0 || y >= len(lines) {
		return
	}
	line := lines[y]
	if x >= 0 && x < len(line) {
		return line, "", true
	}
	return
}

func (c *codeBlock) CharAt(x, y int) (rune, bool) {
	if c.w <= 0 {
		return 0, false
	}
	lines := c.codeLines()
	if y < 0 || y >= len(lines) {
		return 0, false
	}
	line := lines[y]
	if x >= 0 && x < len(line) {
		return rune(line[x]), true
	}
	return 0, false
}

func (c *codeBlock) codeLines() []string {
	lines := strings.Split(c.code, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
