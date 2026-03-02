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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	htmlcomp "github.com/unstablebuild/blue/tui/component/html"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

const (
	barTestWidth  = 80
	barTestHeight = 6 // 3 bar + 3 content
)

func TestBar(t *testing.T) {
	srv := barTestServer(t)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port := u.Port()

	t.Run("no bar by default", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"))
		defer func() { _ = h.Close() }()
		assert.Nil(t, h.bar)
	})

	t.Run("renders at top", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		waitBarLoaded(t, h)

		cases := []handlertest.SequenceTestCase{
			{InputSequence: "", Expected: fmt.Sprintf(
				`          ┌────────────────────────┐    
 %c  %c     │http://127.0.0.1:%s/ │    
          └────────────────────────┘    
Hello                                   
                                        
                                        
                                        
                                        
                                        
                                        `,
				backButton, forwardButton, port)},
		}
		handlertest.RunHandlerSequence(t, h, 40, 10, cases)
	})

	t.Run("updates URL, wraps around", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		waitBarLoaded(t, h)

		_, handled := h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft, MouseX: 70, MouseY: 1})
		require.True(t, handled)

		cases := []handlertest.SequenceTestCase{
			{InputSequence: "abc1234", Expected: `        ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓ 
      ┃4▐                           ┃ 
        ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛ 
Hello                                   
                                        
                                        
                                        
                                        
                                        
                                        `,
			},
		}
		handlertest.RunHandlerSequence(t, h, 40, 10, cases)
	})

	t.Run("renders at bottom", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarBottom))
		defer func() { _ = h.Close() }()
		waitBarLoaded(t, h)

		cases := []handlertest.SequenceTestCase{
			{InputSequence: "", Expected: fmt.Sprintf(
				`Hello                                   
                                        
                                        
                                        
                                        
                                        
                                        
          ┌────────────────────────┐    
 %c  %c     │http://127.0.0.1:%s/ │    
          └────────────────────────┘    `,
				backButton, forwardButton, port)},
		}
		handlertest.RunHandlerSequence(t, h, 40, 10, cases)
	})

	t.Run("URL display", func(t *testing.T) {
		u := srv.URL + "/"
		h := New(term.NopInterrupter(), mustParseURL(t, u),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		assert.Equal(t, u, h.bar.input.Text())
	})

	t.Run("URL updates on navigation", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/p1"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		assert.Equal(t, srv.URL+"/p1", h.bar.input.Text())

		h.navigateTo(mustParseURL(t, srv.URL+"/p2"))
		assert.Equal(t, srv.URL+"/p2", h.bar.input.Text())
	})

	t.Run("URL updates on back/forward", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/p1"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		h.navigateTo(mustParseURL(t, srv.URL+"/p2"))
		assert.Equal(t, srv.URL+"/p2", h.bar.input.Text())

		h.goBack()
		assert.Equal(t, srv.URL+"/p1", h.bar.input.Text())

		h.goForward()
		assert.Equal(t, srv.URL+"/p2", h.bar.input.Text())
	})

	t.Run("click bar focuses", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		assert.False(t, h.bar.focused)

		// Click on the inputbox area (x >= buttonWidth, y in bar range).
		h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseLeft,
			MouseX: 5, MouseY: 1,
		})
		assert.True(t, h.bar.focused)
	})

	t.Run("click outside unfocuses", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		// Focus bar.
		h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseLeft,
			MouseX: 5, MouseY: 1,
		})
		assert.True(t, h.bar.focused)

		// Release from the first click.
		h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseRelease})

		// Click content area.
		h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseLeft,
			MouseX: 5, MouseY: 4,
		})
		assert.False(t, h.bar.focused)
	})

	t.Run("esc unfocuses", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		h.bar.setFocused(true)
		assert.True(t, h.bar.focused)

		exit, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
		assert.False(t, exit, "Esc should NOT exit the handler")
		assert.True(t, handled)
		assert.False(t, h.bar.focused)
	})

	t.Run("ctrl-c unfocuses", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		h.bar.setFocused(true)

		keys, err := term.ParseKeys("<ctrl-c>")
		require.NoError(t, err)
		k := keys[0]
		exit, handled := h.Handle(term.Event{
			Type: term.EventKey, Ch: k.Ch, Mod: k.Mod, Key: k.Key,
		})
		assert.False(t, exit, "Ctrl-C should NOT exit the handler")
		assert.True(t, handled)
		assert.False(t, h.bar.focused)
	})

	t.Run("enter navigates", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/p1"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)
		waitBarLoaded(t, h)

		// Focus bar and set a new URL.
		h.bar.setFocused(true)
		h.bar.setURL(srv.URL + "/p2")

		exit, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
		assert.False(t, exit)
		assert.True(t, handled)
		assert.False(t, h.bar.focused)
		assert.Equal(t, srv.URL+"/p2", h.history[h.historyIdx].String())
	})

	t.Run("cursor hidden when unfocused", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		_, _, show := h.Cursor()
		assert.False(t, show)
	})

	t.Run("cursor visible when focused", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		h.bar.setFocused(true)
		pos, style, show := h.Cursor()
		assert.True(t, show)
		assert.Equal(t, term.CursorStyleSteadyBar, style)
		assert.Equal(t, 1, pos.Y, "cursor Y at bar middle row")
		assert.GreaterOrEqual(t, pos.X, h.bar.frameOffsetX+1,
			"cursor X within frame content area")
	})

	t.Run("selection delegation", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		// Unfocused: no bar selection.
		_, ok := h.Selection()
		assert.False(t, ok)

		// Focused: selection from inputbox (no selection initially).
		h.bar.setFocused(true)
		_, ok = h.Selection()
		assert.False(t, ok)
	})

	t.Run("back button click", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/p1"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)
		waitBarLoaded(t, h)

		h.navigateTo(mustParseURL(t, srv.URL+"/p2"))
		assert.Equal(t, srv.URL+"/p2", h.bar.input.Text())

		// Click ◀ at (0, 1).
		_, handled := h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseLeft,
			MouseX: 0, MouseY: 1,
		})
		assert.True(t, handled)
		assert.Equal(t, srv.URL+"/p1", h.bar.input.Text())
		assert.False(t, h.bar.focused, "button clicks should not focus bar")
	})

	t.Run("forward button click", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/p1"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)
		waitBarLoaded(t, h)

		h.navigateTo(mustParseURL(t, srv.URL+"/p2"))
		h.goBack()
		assert.Equal(t, srv.URL+"/p1", h.bar.input.Text())

		// Click ▶ at (2, 1).
		_, handled := h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseLeft,
			MouseX: 2, MouseY: 1,
		})
		assert.True(t, handled)
		assert.Equal(t, srv.URL+"/p2", h.bar.input.Text())
	})

	t.Run("buttons disabled at boundaries", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/p1"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		h.Resize(barTestWidth, barTestHeight)

		// At start of history, both buttons rendered dim.
		assert.Equal(t, 0, h.historyIdx)
		w := barDraw(t, h)
		assert.Equal(t, tcell.AttrDim, cellAt(w, 1, 1).Attrs, "back dim at start")
		assert.Equal(t, tcell.AttrDim, cellAt(w, 4, 1).Attrs, "forward dim at start")

		// Click ◀: does nothing at start.
		h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseLeft,
			MouseX: 0, MouseY: 1,
		})
		assert.Equal(t, 0, h.historyIdx)

		// Navigate forward: back enabled, forward still dim.
		h.navigateTo(mustParseURL(t, srv.URL+"/p2"))
		assert.Equal(t, 1, h.historyIdx)
		w = barDraw(t, h)
		assert.Equal(t, tcell.AttrNone, cellAt(w, 1, 1).Attrs, "back not dim after nav")
		assert.Equal(t, tcell.AttrDim, cellAt(w, 4, 1).Attrs, "forward dim at end")

		// At end of history, ▶ does nothing.
		h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseLeft,
			MouseX: 2, MouseY: 1,
		})
		assert.Equal(t, 1, h.historyIdx)

		// Go back: back dim, forward enabled.
		h.goBack()
		w = barDraw(t, h)
		assert.Equal(t, tcell.AttrDim, cellAt(w, 1, 1).Attrs, "back dim at start again")
		assert.Equal(t, tcell.AttrNone, cellAt(w, 4, 1).Attrs, "forward not dim with forward history")
	})

	t.Run("double click selects URL word", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		waitBarLoaded(t, h)

		// The URL is "http://127.0.0.1:PORT/". We want to
		// double-click on "127" to select it.
		// With width=80 and PadAutoFloating, the frame is
		// centered in the span. The content offset varies with
		// URL length, but X=39 lands on "127" for any port
		// length (3-5 digits).
		clickX := 39
		clickY := 1 // middle row of bar (bar is at top, rows 0-2)

		// First click: focuses bar, positions cursor.
		h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseLeft,
			MouseX: clickX, MouseY: clickY,
		})
		require.True(t, h.bar.focused)
		// Release.
		h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseRelease,
			MouseX: clickX, MouseY: clickY,
		})
		// Second click: double-click selects word.
		h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseLeft,
			MouseX: clickX, MouseY: clickY,
		})

		sel, ok := h.Selection()
		assert.True(t, ok, "expected a selection after double-click")
		assert.Equal(t, "127", sel)
	})

	t.Run("triple click selects entire URL", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		waitBarLoaded(t, h)

		clickX := 16
		clickY := 1

		for range 3 {
			h.Handle(term.Event{
				Type: term.EventMouse, Key: term.MouseLeft,
				MouseX: clickX, MouseY: clickY,
			})
			h.Handle(term.Event{
				Type: term.EventMouse, Key: term.MouseRelease,
				MouseX: clickX, MouseY: clickY,
			})
		}
		// The triple-click needs the 3rd press (release already sent above
		// but we need to send the 3rd click without a trailing release for
		// the selection to be active).
		h.Handle(term.Event{
			Type: term.EventMouse, Key: term.MouseLeft,
			MouseX: clickX, MouseY: clickY,
		})

		sel, ok := h.Selection()
		assert.True(t, ok, "expected a selection after triple-click")
		assert.Equal(t, srv.URL+"/", sel)
	})

	t.Run("typing widens bar frame", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		waitBarLoaded(t, h)
		h.Resize(40, 10)

		// Initial frame Dimensions from the URL "http://127.0.0.1:PORT/"
		// (23–24 chars depending on port length).
		initW, initH := h.bar.frame.Dimensions()

		// Focus bar and type characters.
		h.bar.setFocused(true)
		for _, ch := range "abcdef" {
			h.bar.inner.Handle(term.Event{Type: term.EventKey, Ch: ch})
		}

		// Frame should have grown by 6 columns (one per typed char).
		newW, newH := h.bar.frame.Dimensions()
		assert.Equal(t, initW+6, newW, "frame should widen by 6 chars")
		assert.Equal(t, initH, newH, "frame height unchanged")

		// Verify the cursor offset also shifted: the span should
		// have re-centered the wider frame, reducing left padding.
		offset := h.bar.inner.ContentOffset()
		assert.Less(t, offset.X, 3,
			"content offset should decrease as frame grows")
	})

	t.Run("keys routed to content when unfocused", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/scroll"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		waitBarLoaded(t, h)

		assert.False(t, h.bar.focused)
		assert.Equal(t, 0, h.SeekOffset())

		// j should scroll down.
		h.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
		assert.Equal(t, 1, h.SeekOffset())

		// k should scroll up.
		h.Handle(term.Event{Type: term.EventKey, Ch: 'k'})
		assert.Equal(t, 0, h.SeekOffset())

		// H/L for history navigation.
		h.navigateTo(mustParseURL(t, srv.URL+"/"))
		assert.Equal(t, 1, h.historyIdx)

		sendBarKeys(t, h, "<shift-h>")
		assert.Equal(t, 0, h.historyIdx)

		sendBarKeys(t, h, "<shift-l>")
		assert.Equal(t, 1, h.historyIdx)
	})

	t.Run("focused bar frame uses highlight charset", func(t *testing.T) {
		h := New(term.NopInterrupter(), mustParseURL(t, srv.URL+"/"),
			WithNavigationBar(BarTop))
		defer func() { _ = h.Close() }()
		waitBarLoaded(t, h)

		unfocused := fmt.Sprintf(
			`          ┌────────────────────────┐    
 %c  %c     │http://127.0.0.1:%s/ │    
          └────────────────────────┘    
Hello                                   
                                        
                                        
                                        
                                        
                                        
                                        `,
			backButton, forwardButton, port)

		focused := fmt.Sprintf(
			`          ┏━━━━━━━━━━━━━━━━━━━━━━━━┓    
 %c  %c     ┃http://127.0.0.1:%s/▐┃    
          ┗━━━━━━━━━━━━━━━━━━━━━━━━┛    
Hello                                   
                                        
                                        
                                        
                                        
                                        
                                        `,
			backButton, forwardButton, port)

		// Unfocused: thin frame.
		h.Resize(40, 10)
		got := handlertest.DrawHandler(h, 40, 10)
		assert.Equal(t, unfocused, got, "unfocused uses thin border")

		// Focus the bar.
		h.bar.setFocused(true)
		got = handlertest.DrawHandler(h, 40, 10)
		assert.Equal(t, focused, got, "focused uses bold border")

		// Unfocus.
		h.bar.setFocused(false)
		got = handlertest.DrawHandler(h, 40, 10)
		assert.Equal(t, unfocused, got, "unfocused again uses thin border")
	})
}

func barTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, "<p>Hello</p>")
		case "/p1":
			_, _ = fmt.Fprintf(w, `<p>Page1</p><p><a href="%s/p2">GoP2</a></p>`, base)
		case "/p2":
			_, _ = fmt.Fprintf(w, `<p>Page2</p><p><a href="%s/p3">GoP3</a></p>`, base)
		case "/p3":
			_, _ = fmt.Fprint(w, `<p>Page3</p>`)
		case "/scroll":
			_, _ = fmt.Fprint(w, "<p>AAA</p><p>BBB</p><p>CCC</p><p>DDD</p><p>EEE</p>")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func waitBarLoaded(t *testing.T, h *Handler) {
	t.Helper()
	waitFor(t, func() bool {
		return h.current.State() == htmlcomp.StateLoaded
	}, 5*time.Second)
	h.Resize(barTestWidth, barTestHeight)
}

func sendBarKeys(t *testing.T, h *Handler, seq string) {
	t.Helper()
	keys, err := term.ParseKeys(seq)
	require.NoError(t, err)
	for _, k := range keys {
		h.Handle(term.Event{Type: term.EventKey, Ch: k.Ch, Mod: k.Mod, Key: k.Key})
	}
}

// barDraw renders the handler into a StringWriter for cell-level inspection.
func barDraw(t *testing.T, h *Handler) *term.StringWriter {
	t.Helper()
	w := term.NewStringWriter(barTestWidth, barTestHeight)
	h.Draw(w)
	return w
}

// cellAt returns the cell at the given screen coordinates.
func cellAt(w *term.StringWriter, x, y int) term.Cell {
	return w.Cells()[y*barTestWidth+x]
}
