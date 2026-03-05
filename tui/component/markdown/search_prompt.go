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

type searchResult int

const (
	searchIgnored searchResult = iota
	searchConsumed
	searchConfirm
	searchCancel
)

// searchPrompt manages a less-style "/" search input bar.
type searchPrompt struct {
	active bool
	buf    []rune
}

func (p *searchPrompt) isActive() bool { return p.active }

func (p *searchPrompt) open() { p.active = true; p.buf = p.buf[:0] }

func (p *searchPrompt) query() string { return string(p.buf) }

func (p *searchPrompt) handleKey(ev term.Event) searchResult {
	if !p.active {
		return searchIgnored
	}

	switch ev.Key {
	case term.KeyEsc:
		p.active = false
		return searchCancel
	case term.KeyEnter:
		p.active = false
		return searchConfirm
	case term.KeyBackspace:
		if len(p.buf) > 0 {
			p.buf = p.buf[:len(p.buf)-1]
		}
		return searchConsumed
	}

	if ev.Ch != 0 {
		p.buf = append(p.buf, ev.Ch)
		return searchConsumed
	}

	return searchConsumed
}

func (p *searchPrompt) draw(w term.Writer, y, width int) {
	if !p.active || width <= 0 {
		return
	}

	for x := range width {
		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
			Ch: ' ', Width: 1,
		})
	}

	w.SetCell(term.Coordinates{X: 0, Y: y}, term.Cell{
		Ch: '/', Width: 1,
		Attributes: term.Attributes{Attrs: tcell.AttrBold},
	})

	for i, r := range p.buf {
		x := i + 1
		if x >= width {
			break
		}
		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
			Ch: r, Width: 1,
		})
	}
}

func (p *searchPrompt) cursor(y int) (term.Coordinates, term.CursorStyle, bool) {
	if !p.active {
		return term.Coordinates{}, term.CursorStyleDefault, false
	}
	return term.Coordinates{X: len(p.buf) + 1, Y: y}, term.CursorStyleDefault, true
}
