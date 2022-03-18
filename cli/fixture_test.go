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
