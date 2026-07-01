// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
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
