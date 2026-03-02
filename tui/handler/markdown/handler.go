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
	"net/url"

	"github.com/unstablebuild/blue/tui/component/markdown"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// Handler wraps a markdown.Component with mouse selection, link handling,
// and less-like keyboard navigation.
//
// Keyboard controls:
//   - Up/k:        Scroll up one line
//   - Down/j:      Scroll down one line
//   - Page Up/b:   Scroll up one page
//   - Page Down/f: Scroll down one page
//   - Space:       Scroll down one page
//   - Home/g:      Go to top
//   - End/G:       Go to bottom
//   - d:           Scroll down half page
//   - u:           Scroll up half page
//   - q/Esc:       Exit (returns exit=true)
//
// It implements handler.ScrollableFloating and handler.Responsive.
type Handler struct {
	comp           *markdown.Component
	mouse          *mouse.Mouse
	selStart       term.Coordinates // document coords
	selEnd         term.Coordinates // document coords
	hasSelection   bool
	width, height  int
	onLinkClick    func(*url.URL) bool
	selectionAttrs term.Attributes
}

var _ handler.ScrollableFloating = (*Handler)(nil)
var _ handler.Responsive = (*Handler)(nil)

// mouseDelegate adapts a Handler to satisfy mouse.Delegate, which requires
// Height() int (viewport height), while the handler's own Height(width int) int
// satisfies handler.Responsive.
type mouseDelegate struct {
	*Handler
}

var _ mouse.Delegate = (*mouseDelegate)(nil)

// Height returns the viewport height for the mouse delegate.
func (d *mouseDelegate) Height() int {
	return d.height
}

// New creates a new markdown handler wrapping the given component.
func New(comp *markdown.Component, opts ...Option) *Handler {
	h := &Handler{
		comp:           comp,
		selectionAttrs: term.Attributes{Attrs: tcell.AttrReverse},
	}
	for _, opt := range opts {
		opt(h)
	}
	h.mouse = mouse.New(&mouseDelegate{h})
	return h
}

// SetComponent swaps the underlying markdown component. Selection
// state is cleared and the component is resized to the handler's
// current dimensions.
func (h *Handler) SetComponent(comp *markdown.Component) {
	h.comp = comp
	h.ClearSelection()
	if h.width > 0 || h.height > 0 {
		h.comp.Resize(h.width, h.height)
	}
}

// Close cancels any in-flight syntax highlighting goroutines
// owned by the underlying component.
func (h *Handler) Close() error {
	return h.comp.Close()
}

// Resize updates the viewport dimensions.
func (h *Handler) Resize(width, height int) {
	h.width = width
	h.height = height
	h.comp.Resize(width, height)
}

// Draw renders the markdown content with selection highlighting.
func (h *Handler) Draw(w term.Writer) {
	h.comp.Draw(w)

	if !h.hasSelection {
		return
	}

	// Overlay selection highlighting
	start, end := term.CoordinatesSort(h.selStart, h.selEnd)
	offset := h.comp.SeekOffset()

	for y := start.Y; y <= end.Y; y++ {
		screenY := y - offset
		if screenY < 0 || screenY >= h.height {
			continue
		}

		lineStart := 0
		lineEnd := h.width
		if y == start.Y {
			lineStart = start.X
		}
		if y == end.Y {
			lineEnd = end.X
		}

		for x := lineStart; x < lineEnd && x < h.width; x++ {
			w.UnionAttributes(term.Coordinates{X: x, Y: screenY}, h.selectionAttrs)
		}
	}
}

// Handle processes keyboard and mouse events.
// Mouse events are delegated to the mouse handler.
// Keyboard events provide less-like navigation.
func (h *Handler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type == term.EventMouse {
		return h.mouse.Handle(ev)
	}

	if ev.Type != term.EventKey {
		return false, false
	}

	switch ev.Key {
	case term.KeyEsc:
		return true, true

	case term.KeyArrowUp:
		h.comp.SeekUp()
		return false, true

	case term.KeyArrowDown:
		h.comp.SeekDown()
		return false, true

	case term.KeyPgup:
		h.scrollPage(-1)
		return false, true

	case term.KeyPgdn:
		h.scrollPage(1)
		return false, true

	case term.KeyHome:
		h.scrollToTop()
		return false, true

	case term.KeyEnd:
		h.scrollToBottom()
		return false, true

	case term.KeySpace:
		h.scrollPage(1)
		return false, true
	}

	switch ev.Ch {
	case 'q':
		return true, true

	case 'k':
		h.comp.SeekUp()
		return false, true

	case 'j':
		h.comp.SeekDown()
		return false, true

	case 'g':
		h.scrollToTop()
		return false, true

	case 'G':
		h.scrollToBottom()
		return false, true

	case 'b': // page up (like less)
		h.scrollPage(-1)
		return false, true

	case 'f': // page down (like less)
		h.scrollPage(1)
		return false, true

	case 'd': // half page down
		h.scrollHalfPage(1)
		return false, true

	case 'u': // half page up
		h.scrollHalfPage(-1)
		return false, true
	}

	return false, false
}

// Cursor returns the cursor position, style, and visibility.
// Markdown handler doesn't show a cursor.
func (h *Handler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

// Selection returns the selected text if any.
func (h *Handler) Selection() (string, bool) {
	if !h.hasSelection {
		return "", false
	}

	start, end := term.CoordinatesSort(h.selStart, h.selEnd)
	if start == end {
		return "", false
	}

	text := h.comp.TextRange(start, end)
	return text, text != ""
}

// SeekUp scrolls the content up.
func (h *Handler) SeekUp() bool {
	return h.comp.SeekUp()
}

// SeekDown scrolls the content down.
func (h *Handler) SeekDown() bool {
	return h.comp.SeekDown()
}

// SeekOffset returns the current scroll offset.
func (h *Handler) SeekOffset() int {
	return h.comp.SeekOffset()
}

// MaxSeekOffset returns the maximum scroll offset.
func (h *Handler) MaxSeekOffset() int {
	return h.comp.MaxSeekOffset()
}

// Dimensions returns the ideal content dimensions.
func (h *Handler) Dimensions() (width, height int) {
	return h.comp.Dimensions()
}

// OnAction handles mouse actions. Returns true to suppress default behavior.
func (h *Handler) OnAction(ev term.Event, pos term.Coordinates, action mouse.Action) bool {
	if action != mouse.LeftClick {
		return false
	}

	link := h.comp.LinkAt(pos.X, pos.Y)
	if link != nil && link.URL != "" {
		parsed, err := url.Parse(link.URL)
		if err != nil {
			return true
		}
		if h.onLinkClick != nil && h.onLinkClick(parsed) {
			return true
		}
		// fallback to local anchors
		if parsed.Fragment != "" && parsed.Scheme == "" && parsed.Host == "" && parsed.Path == "" {
			h.comp.SeekToAnchor(parsed.Fragment)
		}
		return true
	}

	return false
}

// ScrollUp scrolls the content up by n lines.
func (h *Handler) ScrollUp(n int) bool {
	scrolled := false
	for range n {
		if h.comp.SeekUp() {
			scrolled = true
		} else {
			break
		}
	}
	return scrolled
}

// ScrollDown scrolls the content down by n lines.
func (h *Handler) ScrollDown(n int) bool {
	scrolled := false
	for range n {
		if h.comp.SeekDown() {
			scrolled = true
		} else {
			break
		}
	}
	return scrolled
}

// SetSelectionStart sets the start of the selection.
func (h *Handler) SetSelectionStart(pos term.Coordinates) {
	// Convert screen coordinates to document coordinates
	h.selStart = h.screenToDoc(pos)
	h.selEnd = h.selStart
	h.hasSelection = true
}

// SetSelectionEnd sets the end of the selection.
func (h *Handler) SetSelectionEnd(pos term.Coordinates) {
	h.selEnd = h.screenToDoc(pos)
}

// ClearSelection clears any active selection.
func (h *Handler) ClearSelection() {
	h.hasSelection = false
	h.selStart = term.Coordinates{}
	h.selEnd = term.Coordinates{}
}

// SelectWordAt selects the word at the given position.
func (h *Handler) SelectWordAt(pos term.Coordinates) {
	start, end := h.comp.WordBoundsAt(pos.X, pos.Y)
	if start == end {
		return
	}
	h.selStart = h.screenToDoc(start)
	h.selEnd = h.screenToDoc(end)
	h.hasSelection = true
}

// SelectLine selects the entire line at the given y position.
func (h *Handler) SelectLine(y int) {
	h.selStart = h.screenToDoc(term.Coordinates{X: 0, Y: y})
	h.selEnd = h.screenToDoc(term.Coordinates{X: h.width, Y: y})
	h.hasSelection = true
}

// Width returns the current width.
func (h *Handler) Width() int {
	return h.width
}

// Height returns the total height needed to render the markdown content at the
// given width. It delegates to the underlying component's Height method. This
// satisfies the component.Responsive interface.
func (h *Handler) Height(width int) int {
	return h.comp.Height(width)
}

// screenToDoc converts screen coordinates to document coordinates.
func (h *Handler) screenToDoc(pos term.Coordinates) term.Coordinates {
	return term.Coordinates{
		X: pos.X,
		Y: pos.Y + h.comp.SeekOffset(),
	}
}

// scrollPage scrolls by one page. direction is -1 for up, 1 for down.
func (h *Handler) scrollPage(direction int) {
	lines := max(1, h.height)
	if direction > 0 {
		for range lines {
			if !h.comp.SeekDown() {
				break
			}
		}
	} else {
		for range lines {
			if !h.comp.SeekUp() {
				break
			}
		}
	}
}

// scrollHalfPage scrolls by half a page. direction is -1 for up, 1 for down.
func (h *Handler) scrollHalfPage(direction int) {
	lines := max(1, h.height/2)
	if direction > 0 {
		for range lines {
			if !h.comp.SeekDown() {
				break
			}
		}
	} else {
		for range lines {
			if !h.comp.SeekUp() {
				break
			}
		}
	}
}

// scrollToTop scrolls to the beginning of the document.
func (h *Handler) scrollToTop() {
	for h.comp.SeekUp() {
	}
}

// scrollToBottom scrolls to the end of the document.
func (h *Handler) scrollToBottom() {
	for h.comp.SeekDown() {
	}
}
