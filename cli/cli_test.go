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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runFuncCLI struct {
	testCLI
	runFn func(ctx context.Context, args []string) error
}

func (c runFuncCLI) Run(ctx context.Context, args []string) error {
	return c.runFn(ctx, args)
}

func TestParseAndRunCommandErrorPropagation(t *testing.T) {
	errSub := errors.New("subcommand failed")
	sub := runFuncCLI{
		testCLI: testCLI{name: "sub"},
		runFn: func(ctx context.Context, args []string) error {
			return errSub
		},
	}

	tsuite := []struct {
		name        string
		args        []string
		expectedErr error
	}{
		// invalid usage must propagate so the process exits non-zero
		{"unknown command", []string{"nope"}, ErrInvalidArgs},
		{"no command", []string{}, ErrInvalidArgs},
		// help is fully handled and exits successfully
		{"help flag", []string{"-h"}, nil},
		// subcommand errors propagate untouched
		{"subcommand error", []string{"sub"}, errSub},
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case %d: %s", i, tcase.name), func(t *testing.T) {
			root := testCLI{name: "root", commands: []CLI{sub}}
			cmds := map[string]CLI{"sub": sub}
			err := ParseAndRunCommand(
				context.Background(), root, NewFlagSet("root"), cmds, tcase.args)
			assert.Equal(t, tcase.expectedErr, err)
		})
	}
}

func TestParseUsageErrorPropagation(t *testing.T) {
	tsuite := []struct {
		name        string
		expArgs     int
		args        []string
		expectedOK  bool
		expectedErr error
	}{
		{"missing required args", 1, []string{}, false, ErrInvalidArgs},
		{"help flag", 0, []string{"-h"}, false, nil},
		{"valid args", 1, []string{"a"}, true, nil},
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case %d: %s", i, tcase.name), func(t *testing.T) {
			c := testCLI{name: "cmd"}
			_, _, ok, err := ParseUsage(c, NewFlagSet("cmd"), tcase.expArgs, tcase.args)
			require.Equal(t, tcase.expectedErr, err)
			assert.Equal(t, tcase.expectedOK, ok)
		})
	}
}
