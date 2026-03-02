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

type horizontalRuleBlock struct {
	cfg *Config
	w   int // width from last Height call
}

var _ block = (*horizontalRuleBlock)(nil)

func newHorizontalRuleBlock(cfg *Config) *horizontalRuleBlock {
	return &horizontalRuleBlock{cfg: cfg}
}

func (hr *horizontalRuleBlock) Height(width int) int {
	if width <= 0 {
		return 0
	}
	return 2
}

func (hr *horizontalRuleBlock) Resize(width, _ int) {
	hr.w = width
}

func (hr *horizontalRuleBlock) Draw(w term.Writer) {
	if hr.w <= 0 {
		return
	}

	ch := hr.cfg.HorizontalRule
	attr := hr.cfg.HorizontalRuleAttr

	if attr.Bg != tcell.ColorDefault {
		bgAttr := term.Attributes{Bg: attr.Bg}
		for x := range hr.w {
			w.UnionAttributes(term.Coordinates{X: x, Y: 0}, bgAttr)
		}
	}

	for x := range hr.w {
		w.SetCell(term.Coordinates{X: x, Y: 0}, term.Cell{
			Ch:         ch,
			Width:      1,
			Attributes: attr,
		})
	}
}

func (hr *horizontalRuleBlock) Dimensions() (width, height int) {
	return 1, 2
}

func (hr *horizontalRuleBlock) SpanAt(
	x, y int,
) (text, url string, ok bool) {
	return
}

func (hr *horizontalRuleBlock) CharAt(x, y int) (rune, bool) {
	if y == 0 && x >= 0 && x < hr.w {
		return hr.cfg.HorizontalRule, true
	}
	return 0, false
}
