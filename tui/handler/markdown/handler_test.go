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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/tui/component/markdown"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

func TestNew(t *testing.T) {
	comp, err := markdown.New("# Hello")
	require.NoError(t, err)

	h := New(comp)
	require.NotNil(t, h)
	assert.NotNil(t, h.comp)
	assert.NotNil(t, h.mouse)
}

func TestNewWithOptions(t *testing.T) {
	comp, err := markdown.New("# Hello")
	require.NoError(t, err)

	var clicked *url.URL
	h := New(comp,
		WithOnLinkClick(func(u *url.URL) bool {
			clicked = u
			return true
		}),
		WithSelectionAttrs(term.Attributes{Attrs: tcell.AttrBold}),
	)

	require.NotNil(t, h)
	assert.Equal(t, tcell.AttrBold, h.selectionAttrs.Attrs)
	assert.NotNil(t, h.onLinkClick)

	testURL, _ := url.Parse("http://example.com")
	handled := h.onLinkClick(testURL)
	assert.True(t, handled)
	assert.Equal(t, "http://example.com", clicked.String())
}

func TestResize(t *testing.T) {
	comp, err := markdown.New("# Hello")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(80, 24)

	assert.Equal(t, 80, h.Width())
	assert.Equal(t, 24, h.Height())
}

func TestSelectionStartEnd(t *testing.T) {
	comp, err := markdown.New("Hello World")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 5)

	text, ok := h.Selection()
	assert.False(t, ok)
	assert.Empty(t, text)

	h.SetSelectionStart(term.Coordinates{X: 0, Y: 0})
	assert.True(t, h.hasSelection)

	h.SetSelectionEnd(term.Coordinates{X: 5, Y: 0})

	text, ok = h.Selection()
	assert.True(t, ok)
	assert.Equal(t, "Hello", text)
}

func TestClearSelection(t *testing.T) {
	comp, err := markdown.New("Hello World")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 5)

	h.SetSelectionStart(term.Coordinates{X: 0, Y: 0})
	h.SetSelectionEnd(term.Coordinates{X: 5, Y: 0})
	assert.True(t, h.hasSelection)

	h.ClearSelection()
	assert.False(t, h.hasSelection)

	text, ok := h.Selection()
	assert.False(t, ok)
	assert.Empty(t, text)
}

func TestSelectWordAt(t *testing.T) {
	comp, err := markdown.New("Hello World")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 5)

	h.SelectWordAt(term.Coordinates{X: 2, Y: 0})

	text, ok := h.Selection()
	assert.True(t, ok)
	assert.Equal(t, "Hello", text)
}

func TestSelectLine(t *testing.T) {
	comp, err := markdown.New("Hello World")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 5)

	h.SelectLine(0)

	assert.True(t, h.hasSelection)
	assert.Equal(t, 0, h.selStart.X)
	assert.Equal(t, 20, h.selEnd.X)
}

func TestScrolling(t *testing.T) {
	content := "# H1\n\n# H2\n\n# H3\n\n# H4\n\n# H5"
	comp, err := markdown.New(content)
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 3)

	assert.Equal(t, 0, h.SeekOffset())

	assert.True(t, h.ScrollDown(3))
	assert.Equal(t, 3, h.SeekOffset())

	assert.True(t, h.ScrollUp(2))
	assert.Equal(t, 1, h.SeekOffset())

	assert.True(t, h.ScrollUp(10))
	assert.Equal(t, 0, h.SeekOffset())
}

func TestDimensions(t *testing.T) {
	comp, err := markdown.New("# Hello\n\nWorld")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(40, 10)

	w, ht := h.Dimensions()
	assert.Greater(t, w, 0)
	assert.Greater(t, ht, 0)
}

func TestCursor(t *testing.T) {
	comp, err := markdown.New("# Hello")
	require.NoError(t, err)

	h := New(comp)
	_, _, show := h.Cursor()
	assert.False(t, show)
}

func TestLinkClickCallback(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		clickX      int
		clickY      int
		expectCall  bool
		expectedURL string
	}{
		{
			name:        "click on link",
			content:     "[Click me](http://example.com)",
			clickX:      3,
			clickY:      0,
			expectCall:  true,
			expectedURL: "http://example.com",
		},
		{
			name:       "click on non-link",
			content:    "Plain text",
			clickX:     3,
			clickY:     0,
			expectCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, err := markdown.New(tt.content)
			require.NoError(t, err)

			var clickedURL *url.URL
			h := New(comp, WithOnLinkClick(func(u *url.URL) bool {
				clickedURL = u
				return true
			}))
			h.Resize(40, 5)

			pos := term.Coordinates{X: tt.clickX, Y: tt.clickY}
			handled := h.OnAction(term.Event{}, pos, mouse.LeftClick)

			if tt.expectCall {
				assert.True(t, handled)
				assert.Equal(t, tt.expectedURL, clickedURL.String())
			} else {
				assert.False(t, handled)
				assert.Nil(t, clickedURL)
			}
		})
	}
}

func TestLocalAnchorScrolling(t *testing.T) {
	content := "# First Header\n\nSome text with [link](#second-header)\n\n# Second Header\n\nMore text"
	comp, err := markdown.New(content)
	require.NoError(t, err)

	h := New(comp) // no custom onLinkClick, uses default anchor handling
	h.Resize(40, 5)

	initialOffset := h.SeekOffset()

	pos := term.Coordinates{X: 17, Y: 3}
	handled := h.OnAction(term.Event{}, pos, mouse.LeftClick)

	if handled {
		assert.NotEqual(t, initialOffset, h.SeekOffset(), "Anchor should scroll to header")
	}
}

func TestDrawWithSelection(t *testing.T) {
	comp, err := markdown.New("Hello World")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 5)

	h.SetSelectionStart(term.Coordinates{X: 0, Y: 0})
	h.SetSelectionEnd(term.Coordinates{X: 5, Y: 0})

	w := term.NewStringWriter(20, 5)
	require.NotPanics(t, func() {
		h.Draw(w)
	})
}

func TestHandleMouseEvent(t *testing.T) {
	comp, err := markdown.New("Hello World")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 5)

	ev := term.Event{
		Type:   term.EventMouse,
		Key:    term.MouseLeft,
		MouseX: 2,
		MouseY: 0,
	}

	_, handled := h.Handle(ev)
	assert.True(t, handled)
}

func TestHandleNonMouseEvent(t *testing.T) {
	comp, err := markdown.New("Hello World")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 5)

	ev := term.Event{
		Type: term.EventKey,
		Key:  term.KeyEnter,
	}

	_, handled := h.Handle(ev)
	assert.False(t, handled)
}

func TestScrollDownAtMax(t *testing.T) {
	comp, err := markdown.New("Short")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 10)

	assert.Equal(t, 0, h.MaxSeekOffset())

	assert.False(t, h.ScrollDown(1))
}

func TestScrollUpAtMin(t *testing.T) {
	comp, err := markdown.New("Short")
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 10)

	assert.False(t, h.ScrollUp(1))
}

func TestOnActionNonLeftClick(t *testing.T) {
	comp, err := markdown.New("[link](http://example.com)")
	require.NoError(t, err)

	var clicked bool
	h := New(comp, WithOnLinkClick(func(u *url.URL) bool {
		clicked = true
		return true
	}))
	h.Resize(20, 5)

	pos := term.Coordinates{X: 2, Y: 0}
	handled := h.OnAction(term.Event{}, pos, mouse.RightClick)

	assert.False(t, handled)
	assert.False(t, clicked)
}

func TestSelectionPersistsThroughScroll(t *testing.T) {
	content := "# H1\n\n# H2\n\n# H3\n\n# H4\n\n# H5"
	comp, err := markdown.New(content)
	require.NoError(t, err)

	h := New(comp)
	h.Resize(20, 3)

	h.SetSelectionStart(term.Coordinates{X: 0, Y: 0})
	h.SetSelectionEnd(term.Coordinates{X: 5, Y: 0})
	assert.True(t, h.hasSelection)

	h.ScrollDown(3)

	assert.True(t, h.hasSelection)
}

func TestKeyboardScrolling(t *testing.T) {
	content := "Line1\n\nLine2\n\nLine3\n\nLine4\n\nLine5\n\nLine6"
	_, err := markdown.New(content)
	require.NoError(t, err)

	width, height := 10, 3

	t.Run("j scrolls down", func(t *testing.T) {
		comp, _ := markdown.New(content)
		h := New(comp)
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "", Expected: "Line1     \n          \nLine2     "},
			{InputSequence: "j", Expected: "          \nLine2     \n          "},
			{InputSequence: "j", Expected: "Line2     \n          \nLine3     "},
		}
		handlertest.RunHandlerSequence(t, h, width, height, cases)
	})

	t.Run("k scrolls up", func(t *testing.T) {
		comp, _ := markdown.New(content)
		h := New(comp)
		h.Resize(width, height)
		h.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
		h.Handle(term.Event{Type: term.EventKey, Ch: 'j'})

		cases := []handlertest.SequenceTestCase{
			{InputSequence: "", Expected: "Line2     \n          \nLine3     "},
			{InputSequence: "k", Expected: "          \nLine2     \n          "},
			{InputSequence: "k", Expected: "Line1     \n          \nLine2     "},
		}
		handlertest.RunHandlerSequence(t, h, width, height, cases)
	})

	t.Run("arrow keys scroll", func(t *testing.T) {
		comp, _ := markdown.New(content)
		h := New(comp)
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "", Expected: "Line1     \n          \nLine2     "},
			{InputSequence: "<down>", Expected: "          \nLine2     \n          "},
		}
		handlertest.RunHandlerSequence(t, h, width, height, cases)

		cases = []handlertest.SequenceTestCase{
			{InputSequence: "<up>", Expected: "Line1     \n          \nLine2     "},
		}
		handlertest.RunHandlerSequence(t, h, width, height, cases)
	})

	t.Run("g goes to top", func(t *testing.T) {
		comp, _ := markdown.New(content)
		h := New(comp)
		h.Resize(width, height)
		for range 5 {
			h.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
		}

		cases := []handlertest.SequenceTestCase{
			{InputSequence: "g", Expected: "Line1     \n          \nLine2     "},
		}
		handlertest.RunHandlerSequence(t, h, width, height, cases)
	})

	t.Run("G goes to bottom", func(t *testing.T) {
		comp, _ := markdown.New(content)
		h := New(comp)
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "", Expected: "Line1     \n          \nLine2     "},
		}
		handlertest.RunHandlerSequence(t, h, width, height, cases)

		h.Handle(term.Event{Type: term.EventKey, Ch: 'G'})
		assert.Equal(t, h.MaxSeekOffset(), h.SeekOffset())
	})
}

func TestKeyboardPageScrolling(t *testing.T) {
	content := "L1\n\nL2\n\nL3\n\nL4\n\nL5\n\nL6\n\nL7\n\nL8\n\nL9"
	width, height := 5, 3

	t.Run("f scrolls page down", func(t *testing.T) {
		comp, _ := markdown.New(content)
		h := New(comp)
		h.Resize(width, height)

		offsetBefore := h.SeekOffset()
		h.Handle(term.Event{Type: term.EventKey, Ch: 'f'})
		offsetAfter := h.SeekOffset()

		assert.Greater(t, offsetAfter, offsetBefore)
		assert.LessOrEqual(t, offsetAfter-offsetBefore, height)
	})

	t.Run("b scrolls page up", func(t *testing.T) {
		comp, _ := markdown.New(content)
		h := New(comp)
		h.Resize(width, height)

		h.Handle(term.Event{Type: term.EventKey, Ch: 'G'})
		offsetBefore := h.SeekOffset()

		h.Handle(term.Event{Type: term.EventKey, Ch: 'b'})
		offsetAfter := h.SeekOffset()

		assert.Less(t, offsetAfter, offsetBefore)
	})

	t.Run("d scrolls half page down", func(t *testing.T) {
		comp, _ := markdown.New(content)
		h := New(comp)
		h.Resize(width, height)

		offsetBefore := h.SeekOffset()
		h.Handle(term.Event{Type: term.EventKey, Ch: 'd'})
		offsetAfter := h.SeekOffset()

		assert.Greater(t, offsetAfter, offsetBefore)
	})

	t.Run("u scrolls half page up", func(t *testing.T) {
		comp, _ := markdown.New(content)
		h := New(comp)
		h.Resize(width, height)

		h.Handle(term.Event{Type: term.EventKey, Ch: 'G'})
		offsetBefore := h.SeekOffset()

		h.Handle(term.Event{Type: term.EventKey, Ch: 'u'})
		offsetAfter := h.SeekOffset()

		assert.Less(t, offsetAfter, offsetBefore)
	})

	t.Run("space scrolls page down", func(t *testing.T) {
		comp, _ := markdown.New(content)
		h := New(comp)
		h.Resize(width, height)

		offsetBefore := h.SeekOffset()
		h.Handle(term.Event{Type: term.EventKey, Key: term.KeySpace})
		offsetAfter := h.SeekOffset()

		assert.Greater(t, offsetAfter, offsetBefore)
	})
}

func TestKeyboardExit(t *testing.T) {
	comp, err := markdown.New("Hello")
	require.NoError(t, err)
	h := New(comp)
	h.Resize(10, 3)

	t.Run("q exits", func(t *testing.T) {
		exit, handled := h.Handle(term.Event{Type: term.EventKey, Ch: 'q'})
		assert.True(t, exit)
		assert.True(t, handled)
	})

	t.Run("Esc exits", func(t *testing.T) {
		exit, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
		assert.True(t, exit)
		assert.True(t, handled)
	})
}

func TestMouseSelectionIntegration(t *testing.T) {
	comp, err := markdown.New("Hello World")
	require.NoError(t, err)
	h := New(comp)
	width, height := 20, 5
	h.Resize(width, height)

	h.Handle(term.Event{
		Type:   term.EventMouse,
		Key:    term.MouseLeft,
		MouseX: 0,
		MouseY: 0,
	})

	w := term.NewStringWriter(width, height)
	h.Draw(w)
	_ = w.Flush()

	h.Handle(term.Event{
		Type:   term.EventMouse,
		Key:    term.MouseLeft,
		MouseX: 5,
		MouseY: 0,
	})

	text, ok := h.Selection()
	assert.True(t, ok)
	assert.Equal(t, "Hello", text)

	cases := []handlertest.SequenceTestCase{
		{InputSequence: "", Expected: "Hello World         \n                    \n                    \n                    \n                    "},
	}
	handlertest.RunHandlerSequence(t, h, width, height, cases)
}

func TestMouseScrollIntegration(t *testing.T) {
	content := "Line1\n\nLine2\n\nLine3\n\nLine4\n\nLine5"
	comp, err := markdown.New(content)
	require.NoError(t, err)
	h := New(comp)
	width, height := 10, 3
	h.Resize(width, height)

	cases := []handlertest.SequenceTestCase{
		{InputSequence: "", Expected: "Line1     \n          \nLine2     "},
	}
	handlertest.RunHandlerSequence(t, h, width, height, cases)

	h.Handle(term.Event{
		Type: term.EventMouse,
		Key:  term.MouseWheelDown,
	})

	w := term.NewStringWriter(width, height)
	h.Draw(w)
	_ = w.Flush()
	assert.Equal(t, 1, h.SeekOffset())

	h.Handle(term.Event{
		Type: term.EventMouse,
		Key:  term.MouseWheelUp,
	})
	assert.Equal(t, 0, h.SeekOffset())
}
