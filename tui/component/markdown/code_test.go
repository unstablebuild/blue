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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// mockParser implements syntaxapi.Parser for testing. Only Highlight is used.
type mockParser struct {
	highlights []textapi.Location
	err        error
}

func (m *mockParser) Highlight(_ workspaceapi.URI, _ string) (
	iterator.Iterator[textapi.Location], error,
) {
	if m.err != nil {
		return nil, m.err
	}
	return iterator.FromSlice(m.highlights), nil
}

func (m *mockParser) Search(string, []string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (m *mockParser) SearchNode(syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (m *mockParser) Query(workspaceapi.URI, string, []string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (m *mockParser) QueryNode(workspaceapi.URI, syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

// captureSchedule overrides cfg.ScheduleNextTick to capture the goroutine's
// callback via a channel. The returned function blocks until the callback
// arrives and then runs it on the caller's goroutine, eliminating races.
func captureSchedule(cfg *Config) func() {
	ch := make(chan func(), 1)
	cfg.ScheduleNextTick = func(fn func()) bool {
		ch <- fn
		return true
	}
	return func() {
		fn := <-ch
		fn()
	}
}

func TestCodeBlockHighlights(t *testing.T) {
	green := term.Attributes{Fg: tcell.ColorGreen}
	blue := term.Attributes{Fg: tcell.ColorBlue}

	cfg := DefaultConfig()
	cfg.CodeBlock = term.Attributes{Fg: tcell.ColorSilver}
	cfg.Parser = &mockParser{
		highlights: []textapi.Location{
			{
				From: term.Coordinates{X: 0, Y: 0},
				To:   term.Coordinates{X: 4, Y: 0},
				Attr: green,
			},
			{
				From: term.Coordinates{X: 5, Y: 0},
				To:   term.Coordinates{X: 9, Y: 0},
				Attr: blue,
			},
		},
	}
	drain := captureSchedule(&cfg)

	cb := newCodeBlock("go", "func main\n", &cfg)
	drain()

	require.Len(t, cb.cells, 1)
	for x := 0; x < 4; x++ {
		assert.Equal(t, green, cb.cells[0][x].Attributes,
			"cell [0][%d] should be green", x)
	}
	for x := 5; x < 9; x++ {
		assert.Equal(t, blue, cb.cells[0][x].Attributes,
			"cell [0][%d] should be blue", x)
	}
}

func TestCodeBlockHighlightsPreserveBg(t *testing.T) {
	green := term.Attributes{Fg: tcell.ColorGreen}
	bgColor := tcell.ColorGray

	cfg := DefaultConfig()
	cfg.CodeBlock = term.Attributes{Fg: tcell.ColorSilver, Bg: bgColor}
	cfg.Parser = &mockParser{
		highlights: []textapi.Location{
			{
				From: term.Coordinates{X: 0, Y: 0},
				To:   term.Coordinates{X: 2, Y: 0},
				Attr: green,
			},
		},
	}
	drain := captureSchedule(&cfg)

	cb := newCodeBlock("go", "hi\n", &cfg)
	drain()

	require.Len(t, cb.cells, 1)
	assert.Equal(t, tcell.ColorGreen, cb.cells[0][0].Fg)
	assert.Equal(t, bgColor, cb.cells[0][0].Bg)
}

func TestCodeBlockDrawWithHighlights(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CodeBlock = term.Attributes{Fg: tcell.ColorSilver}
	cfg.Parser = &mockParser{
		highlights: []textapi.Location{
			{
				From: term.Coordinates{X: 0, Y: 0},
				To:   term.Coordinates{X: 5, Y: 0},
				Attr: term.Attributes{Fg: tcell.ColorRed},
			},
		},
	}
	drain := captureSchedule(&cfg)

	cb := newCodeBlock("go", "hello\n", &cfg)
	drain()

	w := term.NewStringWriter(10, 2)
	require.NoError(t, w.Clear(term.Attributes{}))

	cb.w = 10
	cb.Draw(w)
	require.NoError(t, w.Flush())

	assert.Equal(t, "hello     \n          ", w.String())
}

func TestCodeBlockDrawWithoutParser(t *testing.T) {
	cfg := DefaultConfig()
	cb := newCodeBlock("go", "hello\n", &cfg)

	w := term.NewStringWriter(10, 2)
	require.NoError(t, w.Clear(term.Attributes{}))

	cb.w = 10
	cb.Draw(w)
	require.NoError(t, w.Flush())

	assert.Equal(t, "hello     \n          ", w.String())

	for x := range 5 {
		assert.Equal(t, cfg.CodeBlock, cb.cells[0][x].Attributes,
			"cell [0][%d] should have CodeBlock attrs", x)
	}
}

func TestCodeBlockDrawEmptyLanguage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Parser = &mockParser{}

	cb := newCodeBlock("", "hello\n", &cfg)

	for x := range 5 {
		assert.Equal(t, cfg.CodeBlock, cb.cells[0][x].Attributes)
	}
}

func TestCodeBlockSpanAt(t *testing.T) {
	cfg := DefaultConfig()
	cb := newCodeBlock("", "hello world\n", &cfg)
	cb.w = 20

	text, url, ok := cb.SpanAt(3, 0)
	assert.True(t, ok)
	assert.Equal(t, "hello world", text)
	assert.Empty(t, url)

	_, _, ok = cb.SpanAt(0, 5)
	assert.False(t, ok)
}

func TestCodeBlockCharAt(t *testing.T) {
	cfg := DefaultConfig()
	cb := newCodeBlock("", "abc\n", &cfg)
	cb.w = 10

	ch, ok := cb.CharAt(0, 0)
	assert.True(t, ok)
	assert.Equal(t, 'a', ch)

	ch, ok = cb.CharAt(2, 0)
	assert.True(t, ok)
	assert.Equal(t, 'c', ch)

	_, ok = cb.CharAt(5, 0)
	assert.False(t, ok)
}

func TestCodeBlockDimensions(t *testing.T) {
	cfg := DefaultConfig()
	cb := newCodeBlock("", "hello\nworld\n", &cfg)

	w, h := cb.Dimensions()
	assert.Equal(t, 5, w)
	assert.Equal(t, 3, h)
}

func TestCodeBlockHighlightsAsync(t *testing.T) {
	green := term.Attributes{Fg: tcell.ColorGreen}

	cfg := DefaultConfig()
	cfg.CodeBlock = term.Attributes{Fg: tcell.ColorSilver}
	cfg.Parser = &mockParser{
		highlights: []textapi.Location{
			{
				From: term.Coordinates{X: 0, Y: 0},
				To:   term.Coordinates{X: 2, Y: 0},
				Attr: green,
			},
		},
	}
	drain := captureSchedule(&cfg)

	cb := newCodeBlock("go", "hi\n", &cfg)

	// Before the callback runs, cells should have base style.
	require.Len(t, cb.cells, 1)
	assert.Equal(t, cfg.CodeBlock, cb.cells[0][0].Attributes)

	// Run the scheduled callback.
	drain()

	// Now cells should reflect the highlight.
	assert.Equal(t, green, cb.cells[0][0].Attributes)
	assert.Equal(t, green, cb.cells[0][1].Attributes)
}

// blockingIterator yields one Location then blocks on Next until the
// context is canceled. When unblocked it closes the done channel.
type blockingIterator struct {
	first    textapi.Location
	yielded  bool
	done     chan struct{}
	closedCh chan struct{}
}

func (it *blockingIterator) Next(ctx context.Context) (textapi.Location, bool) {
	if !it.yielded {
		it.yielded = true
		return it.first, true
	}
	// Block until ctx is canceled.
	<-ctx.Done()
	close(it.done)
	return textapi.Location{}, false
}

func (it *blockingIterator) Err() error { return nil }

func (it *blockingIterator) Close() error {
	close(it.closedCh)
	return nil
}

// blockingParser returns a blockingIterator from Highlight.
type blockingParser struct {
	iter *blockingIterator
}

func (p *blockingParser) Highlight(_ workspaceapi.URI, _ string) (
	iterator.Iterator[textapi.Location], error,
) {
	return p.iter, nil
}

func (p *blockingParser) Search(string, []string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (p *blockingParser) SearchNode(syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (p *blockingParser) Query(workspaceapi.URI, string, []string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (p *blockingParser) QueryNode(workspaceapi.URI, syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func TestCodeBlockCloseCancelsIterator(t *testing.T) {
	it := &blockingIterator{
		first: textapi.Location{
			From: term.Coordinates{X: 0, Y: 0},
			To:   term.Coordinates{X: 2, Y: 0},
			Attr: term.Attributes{Fg: tcell.ColorGreen},
		},
		done:     make(chan struct{}),
		closedCh: make(chan struct{}),
	}

	cfg := DefaultConfig()
	cfg.CodeBlock = term.Attributes{Fg: tcell.ColorSilver}
	cfg.Parser = &blockingParser{iter: it}

	// Use a ScheduleNextTick that never runs the callback,
	// so we can observe the goroutine's lifecycle independently.
	cfg.ScheduleNextTick = func(func()) bool { return true }

	cb := newCodeBlock("go", "hi\n", &cfg)

	// The goroutine is now blocked inside iter.Next waiting
	// for the second element. Cancel via close.
	cb.close()

	// The done channel is closed once the blocked Next returns,
	// proving that Close short-circuited iterator consumption.
	select {
	case <-it.done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("close did not cancel the iterator within 2s")
	}
}

func TestCodeBlockHeightBasic(t *testing.T) {
	cfg := DefaultConfig()
	cb := newCodeBlock("", "a\nb\nc\n", &cfg)

	assert.Equal(t, 4, cb.Height(20))
	assert.Equal(t, 0, cb.Height(0))
}
