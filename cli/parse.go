package cli

import (
	"flag"
)

func isBoolFlag(fs *FlagSet, name string) (is bool) {
	fs.VisitAll(func(f *flag.Flag) {
		if f.Name == name {
			_, is = f.Value.(interface{ IsBoolFlag() bool })
		}
	})
	return
}

func findLastOptionIdx(fs *FlagSet, args []string) (idx int, length int) {
	isNotFlagArg := true
	for i, arg := range args {
		if arg[0] != '-' {
			if isNotFlagArg {
				idx = i
				return
			}
			isNotFlagArg = true
		} else {
			isNotFlagArg = isBoolFlag(fs, arg[1:])
		}
		length++
	}
	return
}

// Parse parses all flags in args and returns the remaining arguments
// as defined by expectedArgs. Note that options are expected to be
// passed first, then arguments: [options] <arg1> <arg2> ...
//
// Example:
//
//   var myBoolOpt
//   fs := NewFlagSet("ie", []string{"my-mandatory-arg"})
//   fs.BoolVar(&myBoolOpt, "bool-opt", false, "bool opt flag")
//
//	 fs.Parse([]string{"-bool-opt"}) // returns nil, ErrInvalidArgs
//	 fs.Parse([]string{"my-value"}) // returns []string{"my-value"}, nil
//	 fs.Parse([]string{"-bool-opt", "my-value"}) // returns []string{"my-value"}, nil
//	 fs.Parse([]string{"-bool-opt", "my-value", "other-values"}) // returns []string{"my-value"}, nil
//
// Note that ErrHelp is returned if -h or --help flags are in args.
func Parse(fs *FlagSet, expectedArgs int, args []string) (
	actualArgs []string, rest []string, err error,
) {
	if args == nil || fs == nil {
		err = ErrInvalidArgs
		return
	}

	_, optsLen := findLastOptionIdx(fs, args)

	if optsLen != 0 {
		options := args[:optsLen]
		err = fs.Parse(options)
		if err == flag.ErrHelp || fs.help {
			err = ErrHelp
		}
		if err != nil {
			return
		}
	}

	if expectedArgs > len(args)-optsLen {
		err = ErrInvalidArgs
		return
	}

	actualArgs = args[optsLen : optsLen+expectedArgs]
	rest = args[optsLen+expectedArgs:]
	return
}

// ParseUsage calls Parse and handles ErrHelp by returning false. If flags are parsed with no
// errors and usage flag is not passed, then this function returns true.
func ParseUsage(cli CLI, fs *FlagSet, expectedArgs int, args []string) (
	rargs []string, ok bool, err error,
) {
	args, _, err = Parse(fs, expectedArgs, args)
	if err != nil {
		if err == ErrHelp {
			Usage(cli)
			err = nil
		}
		return
	}
	ok = true
	rargs = args
	return
}
