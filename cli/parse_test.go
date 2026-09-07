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
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const errFlagNotDefined = "flag provided but not defined"

var (
	indepeFlag bool
	anyFlag    int
)

func emptyFlagSet() *FlagSet {
	return NewFlagSet("Sad Panda")
}

func nonEmptyFlagSet() *FlagSet {
	fs := NewFlagSet("Els Segadors")
	fs.BoolVar(&indepeFlag, "independencia", true, "Ni Oblit, Ni Perdo.")
	fs.IntVar(&anyFlag, "any", 2019, "DUI Ja.")
	return fs
}

func requireError(t *testing.T, err error, expected interface{}) {
	switch expected.(type) {
	case string:
		require.NotNil(t, err)
		require.Contains(t, err.Error(), expected)
	default:
		require.Equal(t, expected, err)
	}
}

func recoverParsePanic(t *testing.T, testCase int) {
	err := recover()
	if err != nil {
		buf := make([]byte, 1<<16)
		runtime.Stack(buf, true)
		t.Fatalf("test case %d panic:%s\n%s\n", testCase, err, buf)
	}
}

func TestParse(t *testing.T) {
	tsuite := []struct {
		// input
		fs      *FlagSet
		expArgs int
		args    []string
		// output
		expectedArgs []string
		expectedRest []string
		expectedErr  interface{}
	}{
		{emptyFlagSet(), 0, nil, nil, nil, ErrInvalidArgs},
		{emptyFlagSet(), 0, []string{}, []string{}, []string{}, nil},
		{emptyFlagSet(), 1, []string{}, nil, nil, ErrInvalidArgs},
		{emptyFlagSet(), 1, []string{"a"}, []string{"a"}, []string{}, nil},
		{emptyFlagSet(), 2, []string{"a"}, nil, nil, ErrInvalidArgs},
		{emptyFlagSet(), 2, []string{"a", "b", "c"}, []string{"a", "b"}, []string{"c"}, nil},
		{emptyFlagSet(), 0, []string{"-opt", "a"}, nil, nil, errFlagNotDefined},
		{nonEmptyFlagSet(), 0, []string{"-opt", "a"}, nil, nil, errFlagNotDefined},
		{nonEmptyFlagSet(), 0, []string{"-independencia", "a"}, []string{}, []string{"a"}, nil},
		{nonEmptyFlagSet(), 2, []string{"a", "-independencia"}, []string{"a", "-independencia"}, []string{}, nil},
		{nonEmptyFlagSet(), 0, []string{"a", "-independencia"}, []string{}, []string{"a", "-independencia"}, nil},
		{nonEmptyFlagSet(), 0, []string{"-any", "2017", "a"}, []string{}, []string{"a"}, nil},
		{nonEmptyFlagSet(), 1, []string{"-independencia", "-any", "2017", "a", "-opt", "a"}, []string{"a"}, []string{"-opt", "a"}, nil},
		{nonEmptyFlagSet(), 1, []string{"-independencia", "-any", "2017", "-opt", "a"}, nil, nil, errFlagNotDefined},
	}

	for i, _tcase := range tsuite {
		tcase := _tcase
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			defer recoverParsePanic(t, i)

			args, rest, err := Parse(tcase.fs, tcase.expArgs, tcase.args)

			requireError(t, err, tcase.expectedErr)
			assert.Equal(t, tcase.expectedArgs, args)
			assert.Equal(t, tcase.expectedRest, rest)
		})
	}
}

func TestParseOptions(t *testing.T) {
	defer recoverParsePanic(t, 0)
	indepeFlag = false
	anyFlag = 0

	_, _, err := Parse(nonEmptyFlagSet(), 0, []string{"-independencia", "-any", "1989"})
	requireError(t, err, nil)
	assert.Equal(t, true, indepeFlag)
	assert.Equal(t, 1989, anyFlag)
}
