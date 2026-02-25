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
)

// LinkInfo contains information about a link at a given position.
type LinkInfo struct {
	URL  string
	Text string
}

type block interface {
	Height(width int) int
	Draw(w term.Writer)
	Dimensions() (width, height int)
	SpanAt(x, y int) (text, url string, ok bool)
	CharAt(x, y int) (rune, bool)
}

type inlineStyle uint8

const (
	styleNone          inlineStyle = 0
	styleBold          inlineStyle = 1 << iota
	styleItalic        inlineStyle = 1 << iota
	styleCode          inlineStyle = 1 << iota
	styleStrikethrough inlineStyle = 1 << iota
	styleLink          inlineStyle = 1 << iota
)

type span struct {
	text  string
	style inlineStyle
	url   string // populated when style includes styleLink
}

type textRun []span

func (tr textRun) String() string {
	var b strings.Builder
	for _, sp := range tr {
		b.WriteString(sp.text)
	}
	return b.String()
}

func (tr textRun) Len() int {
	var n int
	for _, sp := range tr {
		n += len(sp.text)
	}
	return n
}

func spanAtInLine(
	line textRun, x int,
) (text, url string, ok bool) {
	pos := 0
	for _, sp := range line {
		spLen := len(sp.text)
		if x >= pos && x < pos+spLen {
			return sp.text, sp.url, true
		}
		pos += spLen
	}
	return
}

func charAtInLine(line textRun, x int) (rune, bool) {
	pos := 0
	for _, sp := range line {
		for _, r := range sp.text {
			if pos == x {
				return r, true
			}
			pos++
		}
	}
	return 0, false
}

func resolveStyle(
	style inlineStyle, cfg *Config,
) term.Attributes {
	attr := cfg.Paragraph
	if style&styleBold != 0 {
		attr = cfg.Bold
	}
	if style&styleItalic != 0 {
		attr = cfg.Italic
	}
	if style&styleCode != 0 {
		attr = cfg.InlineCode
	}
	if style&styleStrikethrough != 0 {
		attr = cfg.Strikethrough
	}
	if style&styleLink != 0 {
		attr = cfg.Link
	}
	return attr
}
