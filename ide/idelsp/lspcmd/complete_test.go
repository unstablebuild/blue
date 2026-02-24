// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package lspcmd

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

var _ browserapi.Floating = (*completionHandler)(nil)

// stubFloating is a minimal browserapi.Floating for testing
// custom NewSearchList without triggering the default
// *completionHandler injection path.
type stubFloating struct {
	width, height int
}

func (s *stubFloating) Handle(_ term.Event) (bool, bool)                  { return false, false }
func (s *stubFloating) Cursor() (term.Coordinates, term.CursorStyle, bool) { return term.Coordinates{}, term.CursorStyleDefault, false }
func (s *stubFloating) Selection() (string, bool)                          { return "", false }
func (s *stubFloating) Resize(w, h int)                                    { s.width = w; s.height = h }
func (s *stubFloating) Draw(_ term.Writer)                                 {}
func (s *stubFloating) Dimensions() (int, int)                             { return s.width, s.height }
func (s *stubFloating) Close() error                                       { return nil }

func testItems(labels ...string) []semanticapi.CompletionItem {
	items := make([]semanticapi.CompletionItem, len(labels))
	for i, l := range labels {
		items[i] = semanticapi.CompletionItem{Label: l}
	}
	return items
}

func noopEditor() *mockEditor {
	return &mockEditor{
		cellEditorFn: func(_ textapi.Handler) textapi.CellEditor {
			return &mockCellEditor{
				editFn: func(
					_ context.Context,
					_, _ term.Coordinates, _ string,
				) (term.Coordinates, term.Coordinates, string, error) {
					return term.Coordinates{}, term.Coordinates{}, "", nil
				},
			}
		},
	}
}

var noIcons map[semanticapi.CompletionItemKind]string

// TestDefaultIcons verifies that defaultIcons returns
// exactly 25 entries covering all CompletionItemKind values.
func TestDefaultIcons(t *testing.T) {
	icons := defaultIcons()
	assert.Len(t, icons, 25)

	allKinds := []semanticapi.CompletionItemKind{
		semanticapi.CompletionItemKindText,
		semanticapi.CompletionItemKindMethod,
		semanticapi.CompletionItemKindFunction,
		semanticapi.CompletionItemKindConstructor,
		semanticapi.CompletionItemKindField,
		semanticapi.CompletionItemKindVariable,
		semanticapi.CompletionItemKindClass,
		semanticapi.CompletionItemKindInterface,
		semanticapi.CompletionItemKindModule,
		semanticapi.CompletionItemKindProperty,
		semanticapi.CompletionItemKindUnit,
		semanticapi.CompletionItemKindValue,
		semanticapi.CompletionItemKindEnum,
		semanticapi.CompletionItemKindKeyword,
		semanticapi.CompletionItemKindSnippet,
		semanticapi.CompletionItemKindColor,
		semanticapi.CompletionItemKindFile,
		semanticapi.CompletionItemKindReference,
		semanticapi.CompletionItemKindFolder,
		semanticapi.CompletionItemKindEnumMember,
		semanticapi.CompletionItemKindConstant,
		semanticapi.CompletionItemKindStruct,
		semanticapi.CompletionItemKindEvent,
		semanticapi.CompletionItemKindOperator,
		semanticapi.CompletionItemKindTypeParameter,
	}
	for _, kind := range allKinds {
		_, ok := icons[kind]
		assert.True(t, ok, "missing icon for kind %d", kind)
	}
}

// TestFormatLabel verifies icon+label formatting.
func TestFormatLabel(t *testing.T) {
	icons := map[semanticapi.CompletionItemKind]string{
		semanticapi.CompletionItemKindFunction: "F",
	}
	tests := []struct {
		name string
		item semanticapi.CompletionItem
		want string
	}{
		{
			name: "known kind prepends icon",
			item: semanticapi.CompletionItem{
				Label: "foo",
				Kind:  semanticapi.CompletionItemKindFunction,
			},
			want: "F foo",
		},
		{
			name: "unknown kind returns bare label",
			item: semanticapi.CompletionItem{
				Label: "bar",
				Kind:  semanticapi.CompletionItemKindText,
			},
			want: "bar",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := formatLabel(test.item, icons)
			assert.Equal(t, test.want, got)
		})
	}
}

// TestCompletionHandlerRender verifies that text
// content is rendered correctly and remains stable
// across key navigation events.
func TestCompletionHandlerRender(t *testing.T) {
	items := testItems("alpha", "beta")
	ch := newCompletionHandler(items, noIcons, &mockEditor{}, &mockHandler{})
	w, h := ch.Dimensions()
	expected := "alpha  \nbeta   "
	cases := []handlertest.SequenceTestCase{
		{InputSequence: "", Expected: expected},
		{InputSequence: "<down>", Expected: expected},
		{InputSequence: "<up>", Expected: expected},
	}
	handlertest.RunHandlerSequence(t, ch, w, h, cases)
}

func TestCompletionHandlerHandle(t *testing.T) {
	tests := []struct {
		name            string
		giveItems       int
		giveOffset      int
		giveEvent       term.Event
		wantExit        bool
		wantHandled     bool
		wantFocusOffset int
	}{
		{
			name:      "esc exits",
			giveItems: 2,
			giveEvent: term.Event{
				Type: term.EventKey, Key: term.KeyEsc,
			},
			wantExit:    true,
			wantHandled: true,
		},
		{
			name:      "enter exits",
			giveItems: 2,
			giveEvent: term.Event{
				Type: term.EventKey, Key: term.KeyEnter,
			},
			wantExit:    true,
			wantHandled: true,
		},
		{
			name:      "down moves focus",
			giveItems: 3,
			giveEvent: term.Event{
				Type: term.EventKey, Key: term.KeyArrowDown,
			},
			wantHandled:     true,
			wantFocusOffset: 1,
		},
		{
			name:       "up moves focus",
			giveItems:  3,
			giveOffset: 1,
			giveEvent: term.Event{
				Type: term.EventKey, Key: term.KeyArrowUp,
			},
			wantHandled: true,
		},
		{
			name:       "down at bottom stays",
			giveItems:  2,
			giveOffset: 1,
			giveEvent: term.Event{
				Type: term.EventKey, Key: term.KeyArrowDown,
			},
			wantHandled:     true,
			wantFocusOffset: 1,
		},
		{
			name:      "up at top stays",
			giveItems: 2,
			giveEvent: term.Event{
				Type: term.EventKey, Key: term.KeyArrowUp,
			},
			wantHandled: true,
		},
		{
			name:      "ctrl-j moves focus down",
			giveItems: 3,
			giveEvent: term.Event{
				Type: term.EventKey,
				Mod:  term.ModCtrl, Ch: 'j',
			},
			wantHandled:     true,
			wantFocusOffset: 1,
		},
		{
			name:       "ctrl-k moves focus up",
			giveItems:  3,
			giveOffset: 1,
			giveEvent: term.Event{
				Type: term.EventKey,
				Mod:  term.ModCtrl, Ch: 'k',
			},
			wantHandled: true,
		},
		{
			name:      "non-key event ignored",
			giveItems: 2,
			giveEvent: term.Event{Type: term.EventMouse},
		},
		{
			name:      "unknown key not handled",
			giveItems: 2,
			giveEvent: term.Event{
				Type: term.EventKey, Key: term.KeyTab,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			labels := make([]string, test.giveItems)
			for i := range labels {
				labels[i] = "x"
			}
			ch := newCompletionHandler(
				testItems(labels...), noIcons, noopEditor(), &mockHandler{},
			)
			for i := 0; i < test.giveOffset; i++ {
				ch.list.FocusDown()
			}
			exit, handled := ch.Handle(test.giveEvent)
			assert.Equal(t, test.wantExit, exit)
			assert.Equal(t, test.wantHandled, handled)
			assert.Equal(t, test.wantFocusOffset, ch.list.FocusOffset())
		})
	}
}

func TestCompletionHandlerDraw(t *testing.T) {
	tests := []struct {
		name            string
		giveItems       []semanticapi.CompletionItem
		giveFocusOffset int
		wantFg          []tcell.Color
	}{
		{
			name:      "first focused",
			giveItems: testItems("a", "b"),
			wantFg:    []tcell.Color{tcell.ColorWhite, tcell.ColorGray},
		},
		{
			name:            "second focused",
			giveItems:       testItems("a", "b"),
			giveFocusOffset: 1,
			wantFg:          []tcell.Color{tcell.ColorGray, tcell.ColorWhite},
		},
		{
			name:      "single entry focused",
			giveItems: testItems("only"),
			wantFg:    []tcell.Color{tcell.ColorWhite},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ch := newCompletionHandler(
				test.giveItems, noIcons, &mockEditor{}, &mockHandler{},
			)
			for i := 0; i < test.giveFocusOffset; i++ {
				ch.list.FocusDown()
			}
			w, h := ch.Dimensions()
			sw := term.NewStringWriter(w, h)
			ch.Draw(sw)
			require.NoError(t, sw.Flush())

			cells := sw.Cells()
			for i, want := range test.wantFg {
				cell := cells[i*w]
				assert.Equal(t, want, cell.Fg, "entry %d foreground", i)
			}
		})
	}
}

func TestCompletionHandlerDimensions(t *testing.T) {
	tests := []struct {
		name      string
		giveItems []semanticapi.CompletionItem
		wantW     int
		wantH     int
	}{
		{
			name:      "width is max label plus 2",
			giveItems: testItems("short", "longer entry"),
			wantW:     utf8.RuneCountInString("longer entry") + 2,
			wantH:     2,
		},
		{
			name:      "height capped at 15",
			giveItems: make([]semanticapi.CompletionItem, 20),
			wantW:     2,
			wantH:     15,
		},
		{
			name:      "single entry",
			giveItems: testItems("a.go:1"),
			wantW:     8,
			wantH:     1,
		},
		{
			name:      "unicode label uses rune count",
			giveItems: testItems("café"),
			wantW:     6,
			wantH:     1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ch := newCompletionHandler(
				test.giveItems, noIcons, &mockEditor{}, &mockHandler{},
			)
			w, h := ch.Dimensions()
			assert.Equal(t, test.wantW, w)
			assert.Equal(t, test.wantH, h)
		})
	}
}

func TestCompletionHandlerApplyItem(t *testing.T) {
	tests := []struct {
		name      string
		giveItems []semanticapi.CompletionItem
		wantText  string
		wantEdit  bool
		wantStart term.Coordinates
		wantEnd   term.Coordinates
	}{
		{
			name: "text edit applied",
			giveItems: []semanticapi.CompletionItem{
				{
					Label: "foo",
					TextEdit: &semanticapi.TextEdit{
						Range: semanticapi.Range{
							Start: semanticapi.Position{
								Line: 1, Character: 2,
							},
							End: semanticapi.Position{
								Line: 1, Character: 5,
							},
						},
						NewText: "foobar",
					},
				},
			},
			wantText:  "foobar",
			wantEdit:  true,
			wantStart: term.Coordinates{X: 2, Y: 1},
			wantEnd:   term.Coordinates{X: 5, Y: 1},
		},
		{
			name: "insert text used",
			giveItems: []semanticapi.CompletionItem{
				{Label: "fn", InsertText: "func() {}"},
			},
			wantText: "func() {}",
			wantEdit: true,
		},
		{
			name:      "label fallback",
			giveItems: []semanticapi.CompletionItem{{Label: "myVar"}},
			wantText:  "myVar",
			wantEdit:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				editCalled bool
				editText   string
				editStart  term.Coordinates
				editEnd    term.Coordinates
			)
			editor := &mockEditor{
				cellEditorFn: func(_ textapi.Handler) textapi.CellEditor {
					return &mockCellEditor{
						editFn: func(
							_ context.Context,
							s, e term.Coordinates, text string,
						) (term.Coordinates, term.Coordinates, string, error) {
							editCalled = true
							editText = text
							editStart = s
							editEnd = e
							return term.Coordinates{}, term.Coordinates{}, "", nil
						},
					}
				},
			}
			ch := newCompletionHandler(
				test.giveItems, noIcons, editor, &mockHandler{},
			)
			err := ch.applyItem()
			require.NoError(t, err)
			assert.Equal(t, test.wantEdit, editCalled)
			assert.Equal(t, test.wantText, editText)
			assert.Equal(t, test.wantStart, editStart)
			assert.Equal(t, test.wantEnd, editEnd)
		})
	}
}

func TestCompletionHandlerApplyItemError(t *testing.T) {
	errBoom := errors.New("boom")
	editor := &mockEditor{
		cellEditorFn: func(_ textapi.Handler) textapi.CellEditor {
			return &mockCellEditor{
				editFn: func(
					_ context.Context,
					_, _ term.Coordinates, _ string,
				) (term.Coordinates, term.Coordinates, string, error) {
					return term.Coordinates{}, term.Coordinates{}, "", errBoom
				},
			}
		},
	}
	ch := newCompletionHandler(testItems("x"), noIcons, editor, &mockHandler{})
	exit, handled := ch.Handle(term.Event{
		Type: term.EventKey, Key: term.KeyEnter,
	})
	assert.True(t, exit, "should still exit on error")
	assert.True(t, handled, "should still be handled on error")
}

// TestCompletionHandlerDrainChannel verifies that items
// sent to the channel appear in the FocusList after Handle.
func TestCompletionHandlerDrainChannel(t *testing.T) {
	ch := make(chan string, 3)
	ch <- "alpha"
	ch <- "beta"
	ch <- "gamma"
	close(ch)

	handler := defaultNewSearchList(ch, &mockEditor{}, &mockHandler{}).(*completionHandler)

	// Trigger drain via Handle with a non-key event.
	handler.Handle(term.Event{Type: term.EventMouse})

	assert.Equal(t, 3, handler.list.Len())
	assert.Equal(t, 3, handler.height)
	assert.True(t, handler.width >= utf8.RuneCountInString("gamma")+2)
}

// TestCompletionHandlerApplyItemAsync verifies that
// applyItem works after goroutine-delivered addItem.
func TestCompletionHandlerApplyItemAsync(t *testing.T) {
	var editText string
	editor := &mockEditor{
		cellEditorFn: func(_ textapi.Handler) textapi.CellEditor {
			return &mockCellEditor{
				editFn: func(
					_ context.Context,
					_, _ term.Coordinates, text string,
				) (term.Coordinates, term.Coordinates, string, error) {
					editText = text
					return term.Coordinates{}, term.Coordinates{}, "", nil
				},
			}
		},
	}

	ch := make(chan string, 1)
	handler := defaultNewSearchList(ch, editor, &mockHandler{}).(*completionHandler)

	// Simulate goroutine delivering an item.
	item := semanticapi.CompletionItem{
		Label:      "myFunc",
		InsertText: "myFunc()",
	}
	handler.addItem(item)
	ch <- "myFunc"
	close(ch)

	// Drain + apply.
	handler.Handle(term.Event{Type: term.EventMouse})
	err := handler.applyItem()
	require.NoError(t, err)
	assert.Equal(t, "myFunc()", editText)
}

// TestCompletionHandlerTriggerKey verifies that pressing
// the trigger key re-fetches completions and calls
// SetWindowContent via scheduleNextTick.
func TestCompletionHandlerTriggerKey(t *testing.T) {
	triggerKey := term.KeyComb{Mod: term.ModCtrl, Ch: 'l'}

	var completionCalls int
	var lastTriggerKind semanticapi.CompletionTriggerKind
	lsp := &mockLSP{
		completionFn: func(
			_ context.Context, p semanticapi.CompletionParams,
		) (semanticapi.CompletionResult, error) {
			completionCalls++
			lastTriggerKind = p.Context.TriggerKind
			return semanticapi.CompletionResult{
				Items: testItems("result"),
			}, nil
		},
	}

	var setContentCalled bool
	wm := &mockWindowManager{
		setWindowContentFn: func(_ browserapi.Window, _ browserapi.Handler) error {
			setContentCalled = true
			return nil
		},
	}

	icons := map[semanticapi.CompletionItemKind]string{}
	ch := make(chan string, 1)
	close(ch)
	handler := defaultNewSearchList(ch, &mockEditor{}, &mockHandler{}).(*completionHandler)
	handler.win = &mockWindow{id: 1}
	handler.lsp = lsp
	handler.wm = wm
	handler.triggerKey = triggerKey
	handler.icons = icons
	handler.nextKind = semanticapi.CompletionTriggerKindTriggerForIncompleteCompletions
	handler.scheduleNextTick = func(fn func()) bool {
		fn()
		return true
	}

	// First press: should toggle to Invoked and re-fetch.
	ev := term.Event{
		Type: term.EventKey,
		Mod:  triggerKey.Mod,
		Ch:   triggerKey.Ch,
	}
	exit, handled := handler.Handle(ev)
	handler.wg.Wait()

	assert.False(t, exit)
	assert.True(t, handled)
	assert.Equal(t, 1, completionCalls)
	assert.Equal(t, semanticapi.CompletionTriggerKindInvoked, lastTriggerKind)
	assert.True(t, setContentCalled)

	// Second press: should toggle back to IncompleteCompletions.
	setContentCalled = false
	handler.Handle(ev)
	handler.wg.Wait()

	assert.Equal(t, 2, completionCalls)
	assert.Equal(t,
		semanticapi.CompletionTriggerKindTriggerForIncompleteCompletions,
		lastTriggerKind,
	)
	assert.True(t, setContentCalled)
}

// TestCompletionHandlerTriggerKeyNotSet verifies that
// a zero-value TriggerKey is not handled.
func TestCompletionHandlerTriggerKeyNotSet(t *testing.T) {
	ch := make(chan string, 1)
	close(ch)
	handler := defaultNewSearchList(ch, &mockEditor{}, &mockHandler{}).(*completionHandler)

	ev := term.Event{
		Type: term.EventKey,
		Mod:  term.ModCtrl,
		Ch:   'l',
	}
	_, handled := handler.Handle(ev)
	assert.False(t, handled)
}

// TestCompleteHandlerCommandAsync verifies that
// HandleCommand creates the channel, calls NewSearchList,
// wm.Floating, and starts the goroutine.
func TestCompleteHandlerCommandAsync(t *testing.T) {
	items := testItems("alpha", "beta")
	lsp := &mockLSP{
		completionFn: func(
			_ context.Context, _ semanticapi.CompletionParams,
		) (semanticapi.CompletionResult, error) {
			return semanticapi.CompletionResult{Items: items}, nil
		},
	}

	var gotFloating browserapi.Floating
	wm := &mockWindowManager{
		floatingFn: func(
			h browserapi.Floating, _ browserapi.FloatingConfig,
		) (browserapi.Window, error) {
			gotFloating = h
			return &mockWindow{id: 1}, nil
		},
	}

	cfg := DefaultCompleteConfig()
	scheduleNextTick := func(fn func()) bool { fn(); return true }
	h := CompleteHandler(lsp, &mockEditor{}, wm, cfg, scheduleNextTick)

	cmd := textapi.Command{Resource: &mockHandler{}}
	err := h.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	require.NotNil(t, gotFloating)

	// Wait for the goroutine to finish by closing.
	require.NoError(t, gotFloating.Close())
}

// TestCompleteHandlerCustomSearchList verifies that a
// custom NewSearchList receives the channel and its
// floating is shown.
func TestCompleteHandlerCustomSearchList(t *testing.T) {
	items := testItems("alpha")
	lsp := &mockLSP{
		completionFn: func(
			_ context.Context, _ semanticapi.CompletionParams,
		) (semanticapi.CompletionResult, error) {
			return semanticapi.CompletionResult{Items: items}, nil
		},
	}

	var receivedLabels []string
	var mu sync.Mutex
	// Use a dedicated stub to ensure the default
	// *completionHandler injection path is skipped.
	customFloating := &stubFloating{width: 10, height: 1}

	var floatingCalled bool
	wm := &mockWindowManager{
		floatingFn: func(
			_ browserapi.Floating, _ browserapi.FloatingConfig,
		) (browserapi.Window, error) {
			floatingCalled = true
			return &mockWindow{id: 1}, nil
		},
	}

	cfg := CompleteConfig{
		Icons: defaultIcons(),
		NewSearchList: func(
			ch <-chan string, _ textapi.Editor,
			_ textapi.Handler,
		) browserapi.Floating {
			go func() {
				for label := range ch {
					mu.Lock()
					receivedLabels = append(receivedLabels, label)
					mu.Unlock()
				}
			}()
			return customFloating
		},
	}
	scheduleNextTick := func(fn func()) bool { fn(); return true }
	h := CompleteHandler(lsp, &mockEditor{}, wm, cfg, scheduleNextTick)

	cmd := textapi.Command{Resource: &mockHandler{}}
	err := h.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	assert.True(t, floatingCalled)

	// Wait for labels to arrive.
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedLabels) == 1
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, receivedLabels[0], "alpha")
}

// TestCloserFloating verifies that Close cancels the
// context and waits for the goroutine.
func TestCloserFloating(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	inner := &completionHandler{
		list:   component.NewFocusList(),
		width:  10,
		height: 1,
	}
	cf := &closerFloating{
		Floating: inner,
		cancel:   cancel,
	}

	var goroutineExited bool
	cf.wg.Add(1)
	go func() {
		defer cf.wg.Done()
		<-ctx.Done()
		goroutineExited = true
	}()

	err := cf.Close()
	require.NoError(t, err)
	assert.True(t, goroutineExited)
}

func TestCompleteHandlerCommand(t *testing.T) {
	tests := []struct {
		name         string
		giveItems    []semanticapi.CompletionItem
		wantFloating bool
	}{
		{
			name:         "empty results still show floating",
			wantFloating: true,
		},
		{
			name:         "non-empty results show floating",
			giveItems:    testItems("alpha", "beta"),
			wantFloating: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lsp := &mockLSP{
				completionFn: func(
					_ context.Context, _ semanticapi.CompletionParams,
				) (semanticapi.CompletionResult, error) {
					return semanticapi.CompletionResult{
						Items: test.giveItems,
					}, nil
				},
			}
			var floatingCalled bool
			wm := &mockWindowManager{
				floatingFn: func(
					_ browserapi.Floating, _ browserapi.FloatingConfig,
				) (browserapi.Window, error) {
					floatingCalled = true
					return &mockWindow{id: 1}, nil
				},
			}
			cfg := DefaultCompleteConfig()
			scheduleNextTick := func(fn func()) bool { fn(); return true }
			h := CompleteHandler(lsp, &mockEditor{}, wm, cfg, scheduleNextTick)
			cmd := textapi.Command{Resource: &mockHandler{}}
			err := h.HandleCommand(context.Background(), cmd)
			require.NoError(t, err)
			assert.Equal(t, test.wantFloating, floatingCalled)
		})
	}
}
