// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
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
	fmt.Fprint(output, "Commands:\n")
	for _, man := range commands {
		fmt.Fprint(output,
			pad.Right(fmt.Sprintf("  %s", man.Name), padding, " "))
		fmt.Fprintf(output, "  %s\n", man.Summary)
	}
	fmt.Fprint(output, "\n")
}

func printOptionsUsage(output io.Writer, padding int, options FlagSet) {
	fmt.Fprint(output, "Options:\n")
	options.VisitAll(func(f *flag.Flag) {
		fmt.Fprint(output,
			pad.Right(fmt.Sprintf("  -%s", f.Name), padding, " "))
		fmt.Fprintf(output, "  %s [default: %s]\n", f.Usage, f.DefValue)
	})
	fmt.Fprint(output, "\n")
}

func printSummaryUsage(
	output io.Writer, summary, name, synopsis string,
) {
	if summary != "" {
		fmt.Fprintf(output, "%s\n\n", summary)
	}

	fmt.Fprintf(output, "Usage: %s %s", name, synopsis)
	fmt.Fprint(output, "\n\n")
}

// UsageError prints err and the cli Usage.
func UsageError(cli CLI, err error) {
	opts := cli.Man().Options
	output := opts.Output()
	fmt.Fprintf(output, "Error: %s\n\n", err)
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
