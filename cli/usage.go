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
	"flag"
	"fmt"
	"io"

	"github.com/willf/pad"
)

func commandStats(fs FlagSet, cmds []Manual) (
	flagLen int, longestName int,
) {
	fs.VisitAll(func(f *flag.Flag) {
		flagLen++
		if nameLen := len(f.Name); nameLen > longestName {
			longestName = nameLen
		}
	})
	for _, cmd := range cmds {
		if nameLen := len(cmd.Name); nameLen > longestName {
			longestName = nameLen
		}
	}
	return
}

func printCommandsUsage(output io.Writer, padding int, commands []Manual) {
	_, _ = fmt.Fprint(output, "Commands:\n")
	for _, man := range commands {
		_, _ = fmt.Fprint(output,
			pad.Right(fmt.Sprintf("  %s", man.Name), padding, " "))
		_, _ = fmt.Fprintf(output, "  %s\n", man.Summary)
	}
	_, _ = fmt.Fprint(output, "\n")
}

func printOptionsUsage(output io.Writer, padding int, options FlagSet) {
	_, _ = fmt.Fprint(output, "Options:\n")
	options.VisitAll(func(f *flag.Flag) {
		_, _ = fmt.Fprint(output,
			pad.Right(fmt.Sprintf("  -%s", f.Name), padding, " "))
		_, _ = fmt.Fprintf(output, "  %s [default: %s]\n", f.Usage, f.DefValue)
	})
	_, _ = fmt.Fprint(output, "\n")
}

func printSummaryUsage(
	output io.Writer, summary, name, synopsis string,
) {
	if summary != "" {
		_, _ = fmt.Fprintf(output, "%s\n\n", summary)
	}

	_, _ = fmt.Fprintf(output, "Usage: %s %s", name, synopsis)
	_, _ = fmt.Fprint(output, "\n\n")
}

// UsageError prints err and the cli Usage.
func UsageError(cli CLI, err error) {
	opts := cli.Man().Options
	output := opts.Output()
	_, _ = fmt.Fprintf(output, "Error: %s\n\n", err)
	Usage(cli)
}

// Usage is the function called when an error occurs while parsing
// a CLI's arguments or when -h flag is passed.
func Usage(cli CLI) {
	man := cli.Man()
	options := man.Options
	commands := man.Commands
	output := options.Output()
	flagLen, longestOptionName := commandStats(options, commands)
	hasSubcommands := len(commands) != 0
	hasOptions := flagLen != 0

	printSummaryUsage(output, man.Summary, man.Name, man.Synopsis)

	const dashCharacterWidth = 1
	const spaceBetweenDesc = 2
	padding := longestOptionName + dashCharacterWidth + spaceBetweenDesc

	if hasOptions {
		printOptionsUsage(output, padding, options)
	}

	if hasSubcommands {
		printCommandsUsage(output, padding, commands)
	}
}
