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
	"context"

	"github.com/unstablebuild/blue/ide/idelsp/languages"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

type codeBlock struct {
	cells  [][]term.Cell
	cfg    *Config
	w      int // width from last Height call
	cancel context.CancelFunc
}

var _ block = (*codeBlock)(nil)

func newCodeBlock(language, code string, cfg *Config) *codeBlock {
	cells := term.StringToCells(code)
	// Strip trailing empty line (goldmark includes trailing newline).
	if len(cells) > 0 && len(cells[len(cells)-1]) == 0 {
		cells = cells[:len(cells)-1]
	}

	// Apply base CodeBlock attributes to every cell.
	for y := range cells {
		for x := range cells[y] {
			cells[y][x].Attributes = cfg.CodeBlock
		}
	}

	cb := &codeBlock{cells: cells, cfg: cfg}

	if cfg.Parser == nil || language == "" {
		return cb
	}

	ctx, cancel := context.WithCancel(context.Background())
	cb.cancel = cancel

	parser := cfg.Parser
	codeBlock := cfg.CodeBlock
	schedule := cfg.ScheduleNextTick
	go func() {
		hls := collectHighlights(ctx, parser, language, code)
		if len(hls) == 0 {
			return
		}
		schedule(func() {
			applyHighlights(cb.cells, hls, codeBlock)
		})
	}()

	return cb
}

// collectHighlights performs I/O by calling Parser.Highlight and
// collects all resulting locations into a slice. The context controls
// cancellation of the iterator consumption.
func collectHighlights(ctx context.Context, parser syntaxapi.Parser, language, code string) []textapi.Location {
	filename := languages.FilenameForLanguage(language)
	uri, err := workspaceapi.ParseURI("file:///" + filename)
	if err != nil {
		return nil
	}
	iter, err := parser.Highlight(uri, code)
	if err != nil {
		return nil
	}
	defer func() { _ = iter.Close() }()

	var hls []textapi.Location
	for hl, ok := iter.Next(ctx); ok; hl, ok = iter.Next(ctx) {
		hls = append(hls, hl)
	}
	return hls
}

func (c *codeBlock) close() {
	if c.cancel != nil {
		c.cancel()
	}
}

// applyHighlights writes highlight attributes into pre-built cells.
// Must only be called on the event-loop thread (or synchronously
// before the codeBlock is used).
func applyHighlights(cells [][]term.Cell, hls []textapi.Location, codeBlock term.Attributes) {
	for _, hl := range hls {
		for y := hl.From.Y; y <= hl.To.Y && y < len(cells); y++ {
			startX := 0
			if y == hl.From.Y {
				startX = hl.From.X
			}
			endX := len(cells[y])
			if y == hl.To.Y {
				endX = hl.To.X
			}
			for x := startX; x < endX && x < len(cells[y]); x++ {
				cells[y][x].Attributes = hl.Attr
				// Preserve the code block background color.
				if codeBlock.Bg != tcell.ColorDefault {
					cells[y][x].Attributes = term.Attributes(tcell.Style{
						Fg:    hl.Attr.Fg,
						Bg:    codeBlock.Bg,
						Attrs: hl.Attr.Attrs,
					})
				}
			}
		}
	}
}

func (c *codeBlock) Height(width int) int {
	if width <= 0 {
		return 0
	}
	return len(c.cells) + 1
}

func (c *codeBlock) Resize(width, _ int) {
	c.w = width
}

func (c *codeBlock) Draw(w term.Writer) {
	if c.w <= 0 {
		return
	}

	if c.cfg.CodeBlock.Bg != tcell.ColorDefault {
		bgAttr := term.Attributes{Bg: c.cfg.CodeBlock.Bg}
		for y := range len(c.cells) {
			for x := range c.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}

	for y, row := range c.cells {
		for x, cell := range row {
			if x >= c.w {
				break
			}
			w.SetCell(term.Coordinates{X: x, Y: y}, cell)
		}
	}
}

func (c *codeBlock) Dimensions() (width, height int) {
	maxWidth := 0
	for _, row := range c.cells {
		if len(row) > maxWidth {
			maxWidth = len(row)
		}
	}
	return maxWidth, len(c.cells) + 1
}

func (c *codeBlock) SpanAt(x, y int) (text, url string, ok bool) {
	if c.w <= 0 {
		return
	}
	if y < 0 || y >= len(c.cells) {
		return
	}
	row := c.cells[y]
	if x >= 0 && x < len(row) {
		// Return the full line text as the span.
		return cellsToString(row), "", true
	}
	return
}

func (c *codeBlock) CharAt(x, y int) (rune, bool) {
	if c.w <= 0 {
		return 0, false
	}
	if y < 0 || y >= len(c.cells) {
		return 0, false
	}
	row := c.cells[y]
	if x >= 0 && x < len(row) {
		return row[x].Ch, true
	}
	return 0, false
}

func cellsToString(row []term.Cell) string {
	runes := make([]rune, len(row))
	for i, c := range row {
		runes[i] = c.Ch
	}
	return string(runes)
}
