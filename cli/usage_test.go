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
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

const expectedGitAddUsage = `Add file contents to the index

Usage: add [options]

Options:
  -h        Display this message [default: false]
  -version  display cli version [default: false]

`

const expectedGitUsage = `Git is the stupid content tracker

Usage: git [options] <cmd>

Options:
  -h        Display this message [default: false]
  -version  display cli version [default: false]

Commands:
  commit    Record changes to the repository
  add       Add file contents to the index

`

const expectedEmptyUsage = "Usage:  \n\nOptions:\n  -h  Display this message [default: false]\n\n"

func TestUsage(t *testing.T) {

	tsuite := []struct {
		in  testCLI
		out string
	}{
		{emptyManual, expectedEmptyUsage},
		{gitManual, expectedGitUsage},
		{gitAddManual, expectedGitAddUsage},
	}

	for _, tcase := range tsuite {
		var b bytes.Buffer
		tcase.in.options.SetOutput(&b)

		Usage(tcase.in)

		assert.Equal(t, tcase.out, b.String())
	}
}
