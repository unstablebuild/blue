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

package ideshell

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const testWidth = 200

type mockCmdHandler struct {
	handleFn func(context.Context, repl.Command) (
		iterator.Iterator[component.Responsive], error,
	)
	completeFn func(context.Context, string, []string) (
		iterator.Iterator[string], error,
	)
	helpFn func(context.Context, []string) (
		iterator.Iterator[component.Responsive], error,
	)
}

func (m *mockCmdHandler) HandleCommand(
	ctx context.Context, cmd repl.Command,
) (iterator.Iterator[component.Responsive], error) {
	if m.handleFn != nil {
		return m.handleFn(ctx, cmd)
	}
	return iterator.FromSlice[component.Responsive](nil), nil
}

func (m *mockCmdHandler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if m.completeFn != nil {
		return m.completeFn(ctx, cmd, args)
	}
	return iterator.Empty[string](), nil
}

func (m *mockCmdHandler) Help(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if m.helpFn != nil {
		return m.helpFn(ctx, args)
	}
	return toLines("mock help"), nil
}

func collectText(
	t *testing.T,
	iter iterator.Iterator[component.Responsive],
) []string {
	t.Helper()
	ctx := context.Background()
	var lines []string
	for {
		item, ok := iter.Next(ctx)
		if !ok {
			break
		}
		h := item.Height(testWidth)
		if h <= 0 {
			continue
		}
		w := term.NewStringWriter(testWidth, h)
		item.Resize(testWidth, h)
		item.Draw(w)
		_ = w.Flush()
		lines = append(lines, w.String())
	}
	require.NoError(t, iter.Err())
	return lines
}

func TestHandleCommandDispatches(t *testing.T) {
	r := NewRegistry()
	called := false
	r.Register("foo", "do foo", &mockCmdHandler{
		handleFn: func(_ context.Context, cmd repl.Command) (
			iterator.Iterator[component.Responsive], error,
		) {
			called = true
			assert.Equal(t, "foo", cmd.Name)
			assert.Equal(t, []string{"bar"}, cmd.Args)
			return iterator.FromSlice([]component.Responsive{
				toResponsive("ok"),
			}), nil
		},
	})

	ctx := context.Background()
	iter, err := r.HandleCommand(ctx, repl.Command{
		Name: "foo", Args: []string{"bar"},
	})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	assert.True(t, called)
	assert.Len(t, out, 1)
}

func TestHandleCommandUnknown(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()
	_, err := r.HandleCommand(ctx, repl.Command{Name: "nope"})
	assert.True(t, errors.Is(err, repl.ErrNotFound))
}

func TestCompleteCommandNames(t *testing.T) {
	r := NewRegistry()
	r.Register("alpha", "a", &mockCmdHandler{})
	r.Register("beta", "b", &mockCmdHandler{})
	r.Register("apex", "a2", &mockCmdHandler{})

	ctx := context.Background()

	// Prefix "a" matches alpha and apex.
	iter, err := r.Complete(ctx, "a", nil)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "apex"}, got)

	// Prefix "b" matches beta.
	iter2, err := r.Complete(ctx, "b", nil)
	require.NoError(t, err)
	defer func() { _ = iter2.Close() }()
	got2, err := iterator.ToSlice(ctx, iter2)
	require.NoError(t, err)
	assert.Equal(t, []string{"beta"}, got2)

	// Empty prefix matches all, sorted.
	iter3, err := r.Complete(ctx, "", nil)
	require.NoError(t, err)
	defer func() { _ = iter3.Close() }()
	got3, err := iterator.ToSlice(ctx, iter3)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "apex", "beta"}, got3)
}

func TestCompleteDelegatesToHandler(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", "do foo", &mockCmdHandler{
		completeFn: func(
			_ context.Context, cmd string, args []string,
		) (iterator.Iterator[string], error) {
			assert.Equal(t, "foo", cmd)
			assert.Equal(t, []string{"ba"}, args)
			return iterator.FromSlice([]string{"bar", "baz"}), nil
		},
	})

	ctx := context.Background()
	iter, err := r.Complete(ctx, "foo", []string{"ba"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"bar", "baz"}, got)
}

func TestCompleteUnknownCommandReturnsEmpty(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()
	iter, err := r.Complete(ctx, "nope", []string{"x"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestHelpListsCommands(t *testing.T) {
	r := NewRegistry()
	r.Register("alpha", "does alpha", &mockCmdHandler{})
	r.Register("beta", "does beta", &mockCmdHandler{})

	ctx := context.Background()
	iter, err := r.Help(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	require.Len(t, out, 2)
	assert.Contains(t, out[0], "alpha")
	assert.Contains(t, out[0], "does alpha")
	assert.Contains(t, out[1], "beta")
	assert.Contains(t, out[1], "does beta")
}

func TestHelpDelegatesToHandler(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", "do foo", &mockCmdHandler{
		helpFn: func(_ context.Context, args []string) (
			iterator.Iterator[component.Responsive], error,
		) {
			assert.Equal(t, []string{"sub"}, args)
			return toLines("foo sub help"), nil
		},
	})

	ctx := context.Background()
	iter, err := r.Help(ctx, []string{"foo", "sub"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	require.Len(t, out, 1)
	assert.Contains(t, out[0], "foo sub help")
}

func TestHelpUnknownCommand(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()
	_, err := r.Help(ctx, []string{"nope"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command: nope")
}

func TestNewRegistersHelp(t *testing.T) {
	_, r := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
	)

	ctx := context.Background()

	// "help" should be registered.
	iter, err := r.Complete(ctx, "help", nil)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"help"}, got)
}

func TestHelpCommandOutput(t *testing.T) {
	r := NewRegistry()
	registerBaseCommands(r)
	r.Register("foo", "does foo things", &mockCmdHandler{})

	ctx := context.Background()

	// "help" with no args lists all commands.
	iter, err := r.HandleCommand(ctx, repl.Command{
		Name: "help",
	})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	out := collectText(t, iter)
	assert.Len(t, out, 2)
	assert.Contains(t, out[0], "foo")
	assert.Contains(t, out[1], "help")
}

func TestHelpCommandDelegates(t *testing.T) {
	r := NewRegistry()
	registerBaseCommands(r)
	r.Register("foo", "does foo", &mockCmdHandler{
		helpFn: func(_ context.Context, args []string) (
			iterator.Iterator[component.Responsive], error,
		) {
			assert.Empty(t, args)
			return toLines("foo detailed help"), nil
		},
	})

	ctx := context.Background()
	iter, err := r.HandleCommand(ctx, repl.Command{
		Name: "help", Args: []string{"foo"},
	})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	require.Len(t, out, 1)
	assert.Contains(t, out[0], "foo detailed help")
}

func TestHelpCommandComplete(t *testing.T) {
	r := NewRegistry()
	registerBaseCommands(r)
	r.Register("foo", "f", &mockCmdHandler{})
	r.Register("far", "f", &mockCmdHandler{})

	ctx := context.Background()

	// Complete "help f<TAB>" should suggest foo and far.
	iter, err := r.Complete(ctx, "help", []string{"f"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"far", "foo"}, got)
}
