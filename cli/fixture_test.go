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
)

type testCLI struct {
	name     string
	summary  string
	synopsis string
	commands []CLI
	options  *FlagSet
}

func (t testCLI) Man() Manual {
	var subMan []Manual
	for _, cmd := range t.commands {
		subMan = append(subMan, cmd.Man())
	}
	return Manual{
		Name:     t.name,
		Summary:  t.summary,
		Synopsis: t.synopsis,
		Commands: subMan,
		Options:  *t.opts(),
	}
}

func (t testCLI) opts() *FlagSet {
	if t.options == nil {
		t.options = NewFlagSet("<default-flagset>")
	}
	return t.options
}

func (t testCLI) Run(ctx context.Context, args []string) error {
	panic("not implemented")
}

var (
	gitFlagSet       = NewFlagSet("git")
	gitLogFlagSet    = NewFlagSet("git-log")
	gitCommitFlagSet = NewFlagSet("git-commit")
)

var (
	emptyManual = testCLI{options: NewFlagSet("empty")}

	gitCommitManual = testCLI{
		name:    "commit",
		summary: "Record changes to the repository",
		options: gitLogFlagSet,
	}

	gitAddManual = testCLI{
		name:     "add",
		synopsis: "[options]",
		summary:  "Add file contents to the index",
		options:  gitCommitFlagSet,
	}

	gitManual = testCLI{
		name:     "git",
		summary:  "Git is the stupid content tracker",
		synopsis: "[options] <cmd>",
		options:  gitFlagSet,
		commands: []CLI{
			gitCommitManual,
			gitAddManual,
		},
	}
)

func init() {
	flagsets := []*FlagSet{gitFlagSet, gitLogFlagSet, gitCommitFlagSet}
	for _, fs := range flagsets {
		var b bool // discard
		fs.BoolVar(&b, "version", false, "display cli version")
	}
}
