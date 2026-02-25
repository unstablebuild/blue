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
	"unicode"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// Component renders markdown content in a terminal.
// It implements component.ScrollableFloating.
type Component struct {
	blocks       []block
	blockHeights []int // cached heights for each block at current width
	anchors      map[string]int // anchor slug -> block index

	width, height int
	offset        int
	totalHeight   int
}

var _ component.ScrollableFloating = (*Component)(nil)

// New creates a new markdown component with the given content
// using default styling. Returns an error if parsing fails.
func New(content string) (*Component, error) {
	return NewWithConfig(content, DefaultConfig())
}

// NewWithConfig creates a new markdown component with the given content
// and custom configuration. Returns an error if parsing fails.
func NewWithConfig(content string, cfg Config) (*Component, error) {
	blocks, err := parse(content, &cfg)
	if err != nil {
		return nil, err
	}
	c := &Component{
		blocks:  blocks,
		anchors: make(map[string]int),
	}
	c.buildAnchors()
	return c, nil
}

// Anchor converts header text to a URL anchor slug.
// It lowercases text, replaces spaces with hyphens,
// and removes non-alphanumeric chars.
func Anchor(text string) string {
	var b strings.Builder
	prev := '-'
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prev = r
		} else if prev != '-' {
			b.WriteRune('-')
			prev = '-'
		}
	}
	result := b.String()
	return strings.Trim(result, "-")
}

// Resize updates the viewport dimensions and recalculates
// block heights if the width has changed.
func (c *Component) Resize(width, height int) {
	if width != c.width {
		c.recalculateHeights(width)
	}
	c.width, c.height = width, height
	if max := c.MaxSeekOffset(); c.offset > max {
		c.offset = max
	}
}

// Draw renders the visible portion of the markdown content.
func (c *Component) Draw(w term.Writer) {
	if c.width <= 0 || c.height <= 0 {
		return
	}
	w = term.BoundsCheckWriter(c.width, c.height, w)
	y := -c.offset
	for i, blk := range c.blocks {
		blockHeight := c.blockHeights[i]
		if y+blockHeight <= 0 {
			y += blockHeight
			continue
		}
		if y >= c.height {
			break
		}
		vw := &component.VirtualWriter{
			Writer: w,
			Offset: term.Coordinates{X: 0, Y: y},
			Width:  c.width,
			Height: blockHeight,
		}
		blk.Draw(vw)
		y += blockHeight
	}
}

// SeekUp scrolls the content up by one row.
func (c *Component) SeekUp() bool {
	if c.offset > 0 {
		c.offset--
		return true
	}
	return false
}

// SeekDown scrolls the content down by one row.
func (c *Component) SeekDown() bool {
	if c.offset < c.MaxSeekOffset() {
		c.offset++
		return true
	}
	return false
}

// SeekOffset returns the current scroll offset.
func (c *Component) SeekOffset() int {
	return c.offset
}

// MaxSeekOffset returns the maximum scroll offset.
func (c *Component) MaxSeekOffset() int {
	if c.totalHeight <= c.height {
		return 0
	}
	return c.totalHeight - c.height
}

// Dimensions returns the ideal width and height
// without wrapping or clipping.
func (c *Component) Dimensions() (width, height int) {
	maxWidth := 0
	totalHeight := 0
	for _, blk := range c.blocks {
		w, h := blk.Dimensions()
		if w > maxWidth {
			maxWidth = w
		}
		totalHeight += h
	}
	return maxWidth, totalHeight
}

// LinkAt returns link information at the given screen
// coordinates relative to the component's viewport.
// Returns nil if no link is found at that position.
func (c *Component) LinkAt(x, y int) *LinkInfo {
	docY := y + c.offset
	blockY := 0
	for i, blk := range c.blocks {
		blockHeight := c.blockHeights[i]
		if docY >= blockY && docY < blockY+blockHeight {
			relY := docY - blockY
			_, url, ok := blk.SpanAt(x, relY)
			if ok && url != "" {
				text, _, _ := blk.SpanAt(x, relY)
				return &LinkInfo{URL: url, Text: text}
			}
			return nil
		}
		blockY += blockHeight
	}
	return nil
}

// SpanAt returns the text and URL at the given screen
// coordinates relative to the component's viewport.
func (c *Component) SpanAt(x, y int) (text, url string, ok bool) {
	docY := y + c.offset
	blockY := 0
	for i, blk := range c.blocks {
		blockHeight := c.blockHeights[i]
		if docY >= blockY && docY < blockY+blockHeight {
			relY := docY - blockY
			return blk.SpanAt(x, relY)
		}
		blockY += blockHeight
	}
	return
}

// SeekToAnchor scrolls to the header with the given
// anchor slug. Returns true if the anchor was found
// and scrolling occurred.
func (c *Component) SeekToAnchor(anchor string) bool {
	blockIdx, ok := c.anchors[anchor]
	if !ok {
		return false
	}

	y := 0
	for i := 0; i < blockIdx && i < len(c.blockHeights); i++ {
		y += c.blockHeights[i]
	}

	if y > c.MaxSeekOffset() {
		y = c.MaxSeekOffset()
	}
	if y == c.offset {
		return false
	}
	c.offset = y
	return true
}

// TextRange extracts text from the given coordinate range.
// Coordinates are in document space (not screen space).
// The range is inclusive of start and exclusive of end.
func (c *Component) TextRange(
	start, end term.Coordinates,
) string {
	if c.width <= 0 {
		return ""
	}

	start, end = term.CoordinatesSort(start, end)
	if start == end {
		return ""
	}

	var result strings.Builder
	docY := 0

	for i, blk := range c.blocks {
		blockHeight := c.blockHeights[i]
		blockEnd := docY + blockHeight

		if blockEnd <= start.Y {
			docY = blockEnd
			continue
		}
		if docY > end.Y || (docY == end.Y && end.X == 0) {
			break
		}

		for lineY := docY; lineY < blockEnd; lineY++ {
			if lineY < start.Y {
				continue
			}
			if lineY > end.Y || (lineY == end.Y && end.X == 0) {
				break
			}

			relY := lineY - docY
			startX := 0
			endX := c.width

			if lineY == start.Y {
				startX = start.X
			}
			if lineY == end.Y {
				endX = end.X
			}

			for x := startX; x < endX; x++ {
				ch, ok := blk.CharAt(x, relY)
				if ok && ch != 0 {
					result.WriteRune(ch)
				}
			}

			if lineY < end.Y-1 ||
				(lineY == end.Y-1 && end.X > 0) {
				result.WriteByte('\n')
			}
		}
		docY = blockEnd
	}

	return result.String()
}

// WordBoundsAt returns the start and end coordinates of
// the word at the given screen position. Returns zero
// coordinates if no word is found.
func (c *Component) WordBoundsAt(
	x, y int,
) (start, end term.Coordinates) {
	text, _, ok := c.SpanAt(x, y)
	if !ok || text == "" {
		return
	}

	startX := x
	for sx := x - 1; sx >= 0; sx-- {
		t, _, ok := c.SpanAt(sx, y)
		if !ok || t != text {
			break
		}
		startX = sx
	}

	relX := x - startX
	if relX < 0 || relX >= len(text) {
		return
	}

	wordStart := relX
	for wordStart > 0 && isWordCharMd(rune(text[wordStart-1])) {
		wordStart--
	}

	wordEnd := relX
	for wordEnd < len(text) && isWordCharMd(rune(text[wordEnd])) {
		wordEnd++
	}

	if !isWordCharMd(rune(text[relX])) {
		for i := relX + 1; i < len(text); i++ {
			if isWordCharMd(rune(text[i])) {
				wordStart = i
				wordEnd = i + 1
				for wordEnd < len(text) &&
					isWordCharMd(rune(text[wordEnd])) {
					wordEnd++
				}
				break
			}
		}
	}

	start = term.Coordinates{X: startX + wordStart, Y: y}
	end = term.Coordinates{X: startX + wordEnd, Y: y}
	return
}

// Close cancels any in-flight syntax highlighting goroutines.
func (c *Component) Close() error {
	for _, blk := range c.blocks {
		if cb, ok := blk.(*codeBlock); ok {
			cb.close()
		}
	}
	return nil
}

func (c *Component) buildAnchors() {
	for i, blk := range c.blocks {
		if h, ok := blk.(*headerBlock); ok {
			anchor := Anchor(h.content.String())
			if anchor != "" {
				c.anchors[anchor] = i
			}
		}
	}
}

func (c *Component) recalculateHeights(width int) {
	c.blockHeights = make([]int, len(c.blocks))
	c.totalHeight = 0
	for i, blk := range c.blocks {
		h := blk.Height(width)
		c.blockHeights[i] = h
		c.totalHeight += h
	}
}

func isWordCharMd(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
