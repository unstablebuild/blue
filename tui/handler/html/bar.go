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

package html

import (
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/handler/inputbox"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

const (
	buttonWidth   = 5
	barHeight     = 3
	backButton    = '\uf0a8'
	forwardButton = '\uf0a9'
)

type clickResult int

const (
	clickNone clickResult = iota
	clickBack
	clickForward
	clickInput
)

// barButtons draws the back/forward navigation buttons.
type barButtons struct {
	backDim, fwdDim bool
	width, height   int
}

func (b *barButtons) Resize(width, height int) {
	b.width = width
	b.height = height
}

func (b *barButtons) Draw(w term.Writer) {
	if b.height < 2 {
		return
	}
	backCell := term.Cell{Ch: backButton, Width: 2}
	if b.backDim {
		backCell.Attributes = term.Attributes{Attrs: tcell.AttrDim}
	}
	w.SetCell(term.Coordinates{X: 1, Y: 1}, backCell)

	fwdCell := term.Cell{Ch: forwardButton, Width: 2}
	if b.fwdDim {
		fwdCell.Attributes = term.Attributes{Attrs: tcell.AttrDim}
	}
	w.SetCell(term.Coordinates{X: 4, Y: 1}, fwdCell)
}

func (b *barButtons) Height(int) int {
	return barHeight
}

// navigationBar is the URL input bar with back/forward buttons.
type navigationBar struct {
	buttons    *barButtons
	inner      *handler.Span
	frame      *handler.Frame
	input      *inputbox.Handler
	focused    bool
	width      int
	frameWidth int

	// frameOffsetX caches the X offset of the frame within the bar
	// so cursor coordinates can be translated without recomputing.
	frameOffsetX int
}

var spanConfig = component.SpanConfig{
	ContentAlignment: component.AlignmentCentered,
	PadAutoFloating:  true,
}

func newNavigationBar(urlStr string) *navigationBar {
	nb := &navigationBar{
		buttons: &barButtons{backDim: true, fwdDim: true},
	}
	nb.input = inputbox.New(inputbox.WithText(urlStr))
	nb.frame = handler.NewFrame(nb.input)
	nb.inner = handler.NewSpan(nb.frame, spanConfig)
	nb.setURL(urlStr)
	return nb
}

// setURL replaces the inputbox with one pre-filled with urlStr.
func (nb *navigationBar) setURL(urlStr string) {
	nb.input = inputbox.New(inputbox.WithText(urlStr))
	nb.frame.Init(nb.input)
	nb.inner.Init(nb.frame, spanConfig)

	if nb.focused {
		nb.frame.FrameCharSet = component.FrameCharSetHighlight()
	}
	if nb.width > 0 {
		nb.inner.Resize(nb.frameWidth, barHeight)
	}
}

func (nb *navigationBar) setFocused(focused bool) {
	nb.focused = focused
	if focused {
		nb.frame.FrameCharSet = component.FrameCharSetHighlight()
	} else {
		nb.frame.FrameCharSet = component.FrameCharSetDefault()
	}
}

func (nb *navigationBar) Resize(width int) {
	nb.width = width
	nb.frameOffsetX = buttonWidth + 3
	nb.frameWidth = width - nb.frameOffsetX - 1
	nb.frameWidth = max(3, nb.frameWidth)
	nb.buttons.Resize(buttonWidth, barHeight)
	nb.inner.Resize(nb.frameWidth, barHeight)
}

func (nb *navigationBar) Draw(w term.Writer) {
	nb.buttons.Draw(w)
	frameW := &component.VirtualWriter{
		Writer: w,
		Offset: term.Coordinates{X: nb.frameOffsetX, Y: 0},
		Width:  nb.frameWidth,
		Height: barHeight,
	}
	nb.inner.Draw(frameW)
}

func (nb *navigationBar) handleClick(x, y int) clickResult {
	if y == 1 {
		if x <= 1 {
			return clickBack
		}
		if x == 2 || x == 3 {
			return clickForward
		}
	}
	if x >= buttonWidth {
		return clickInput
	}
	return clickNone
}

func (nb *navigationBar) handleMouse(ev term.Event) (bool, bool) {
	ev.MouseX -= nb.frameOffsetX
	return nb.inner.Handle(ev)
}

func (nb *navigationBar) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	if !nb.focused {
		return term.Coordinates{}, term.CursorStyleDefault, false
	}
	pos, style, show := nb.inner.Cursor()
	pos.X += nb.frameOffsetX
	return pos, style, show
}

func (nb *navigationBar) Selection() (string, bool) {
	if !nb.focused {
		return "", false
	}
	return nb.inner.Selection()
}
