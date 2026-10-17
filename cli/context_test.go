// Copyright 2018-2026 Unstable Build, LLC
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

package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newParsedFlagSet(t *testing.T) *FlagSet {
	fs := NewFlagSet("beteve")
	fs.Bool("reapers", true, "Revolution")
	fs.Bool("police", true, "Repression")

	err := fs.Parse([]string{"-reapers", "-police"})
	require.NoError(t, err)

	return fs
}

func TestPopulateContext(t *testing.T) {
	ctx := context.Background()
	fs := newParsedFlagSet(t)

	ctx = ContextWithOptions(ctx, fs)

	reapers, ok1 := OptionFromContext(ctx, "reapers")
	police, ok2 := OptionFromContext(ctx, "police")

	assert.Equal(t, true, ok1)
	assert.Equal(t, true, ok2)
	assert.Equal(t, true, reapers)
	assert.Equal(t, true, police)
}

func TestDoesNotConflict(t *testing.T) {
	ctx := context.Background()
	fs := newParsedFlagSet(t)

	ctx = ContextWithOptions(ctx, fs)

	type myKey string
	var reaper myKey = "reapers"
	var sickles myKey = "sickles"
	ctx = context.WithValue(ctx, reaper, false)
	ctx = context.WithValue(ctx, sickles, true)

	reapers, _ := OptionFromContext(ctx, "reapers")
	assert.Equal(t, true, reapers)

	_, ok := OptionFromContext(ctx, "sickles")
	assert.Equal(t, false, ok)
}

func TestDispatchHook(t *testing.T) {
	group := func(name string, cmds map[string]CLI) CLI {
		return runFuncCLI{
			testCLI: testCLI{name: name},
			runFn: func(ctx context.Context, args []string) error {
				return ParseAndRunCommand(ctx, testCLI{name: name},
					NewFlagSet(name), cmds, args)
			},
		}
	}
	leaf := runFuncCLI{
		testCLI: testCLI{name: "leaf"},
		runFn:   func(context.Context, []string) error { return nil },
	}
	cmds := map[string]CLI{
		"release": group("release", map[string]CLI{
			"upload": leaf,
			"get":    leaf,
		}),
		"init": leaf,
	}

	tsuite := []struct {
		name string
		args []string
		want [][]string
	}{
		{"leaf", []string{"init", "arg"}, [][]string{{"init"}}},
		{"nested", []string{"release", "upload", "a", "b"},
			[][]string{{"release"}, {"release", "upload"}}},
		{"unknown sub-command", []string{"release", "nope"},
			[][]string{{"release"}}},
		{"unknown command", []string{"nope"}, nil},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			var got [][]string
			ctx := ContextWithDispatchHook(context.Background(),
				func(path []string) { got = append(got, path) })

			_ = RunCommand(ctx, tcase.args, cmds)

			assert.Equal(t, tcase.want, got)
		})
	}
}

func TestDispatchHookSiblingPaths(t *testing.T) {
	var got [][]string
	ctx := ContextWithDispatchHook(context.Background(),
		func(path []string) { got = append(got, path) })
	leaf := runFuncCLI{runFn: func(context.Context, []string) error { return nil }}
	cmds := map[string]CLI{"a": leaf, "b": leaf}
	// three levels deep, appending to the path would have spare capacity
	for _, name := range []string{"x", "y", "z"} {
		ctx = contextWithDispatched(ctx, name)
	}

	require.NoError(t, RunCommand(ctx, []string{"a"}, cmds))
	require.NoError(t, RunCommand(ctx, []string{"b"}, cmds))

	assert.Equal(t, []string{"x", "y", "z", "a"}, got[3])
	assert.Equal(t, []string{"x", "y", "z", "b"}, got[4])
}

func TestRunCommandWithoutDispatchHook(t *testing.T) {
	leaf := runFuncCLI{runFn: func(context.Context, []string) error { return nil }}
	require.NoError(t, RunCommand(context.Background(), []string{"a"},
		map[string]CLI{"a": leaf}))
}
