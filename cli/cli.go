package cli

import (
	"context"
	"flag"
	"os"
)

// Manual represents a CLI's manual and documentation.
type Manual struct {
	Name string

	// Summary is a short 80-100 character description.
	Summary string

	// Synopsis is a single line synopsis of how
	// this CLI is to be used. It should ONLY include
	// the semantic information about how arguments are parsed.
	//
	// Example: [<options>] [<revision-range>] [[--] <path>...]
	Synopsis string

	// Commands is a list of accepted commands or nil
	// if no commands are expected.
	Commands []Manual

	Options FlagSet
}

// CLI represents a command-line interface.
type CLI interface {
	// Parse takes args and interprets them.
	Run(ctx context.Context, args []string) error

	Man() Manual
}

// FlagSet is just a type alias used to foster NewFlagSet usage.
type FlagSet struct {
	flag.FlagSet
	help bool
}

// Exit prints r's usage and exits the program exit code.
func Exit(r CLI, code int) {
	Usage(r)
	os.Exit(code)
}

// Run runs cli with os.Args
func Run(ctx context.Context, cli CLI) error {
	args := os.Args[1:]
	return cli.Run(ctx, args)
}

// RunCommand is a helper that attempts to find and run the next command in cmds.
func RunCommand(ctx context.Context, args []string, cmds map[string]CLI) error {
	if len(args) == 0 {
		return ErrInvalidArgs
	}
	argCmd := args[0]
	if cmd, ok := cmds[argCmd]; ok {
		return cmd.Run(ctx, args[1:])
	}

	return ErrInvalidArgs
}

// ParseAndRunCommand is a helper to run CLI implementations
// that expect no arguments and simply run a sub-command CLI.
//
// Options parsed are propagated to sub-command CLI via context.Context
// and can be retrieved by using OptionFromContext.
func ParseAndRunCommand(
	ctx context.Context, c CLI, fs *FlagSet, cmds map[string]CLI, args []string,
) error {
	_, rest, err := Parse(fs, 0, args)
	if err != nil {
		if err == ErrHelp {
			Usage(c)
			err = nil
		}
		return err
	}

	ctx = ContextWithOptions(ctx, fs)

	return RunCommand(ctx, rest, cmds)
}

// NewFlagSet is a helper around flag.NewFlagSet that sets sane defaults.
func NewFlagSet(name string) *FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	f := &FlagSet{FlagSet: *fs}
	f.BoolVar(&f.help, "h", false, "Display this message")

	return f
}
