// Copyright 2026 Unstable Build, LLC.
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

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/unstablebuild/blue/tui/sh"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/inputbox"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

var commands = []string{
	"date", "greet", "help", "repeat", "rev", "upper",
}

type handler struct{}

func (handler) HandleCommand(
	_ context.Context, cmd repl.Command,
) (iterator.Iterator[component.Responsive], error) {
	switch cmd.Name {
	case "help":
		return lines(
			"Available commands: "+strings.Join(commands, ", "),
			"",
			"Shell features supported via sh interpreter:",
			"  pipes:      greet world | upper",
			"  semicolons: greet a; greet b",
			"  variables:  NAME=Go; greet $NAME",
			"  logic:      true && greet yes",
			"  subshells:  (greet sub)",
		), nil
	case "greet":
		name := "world"
		if len(cmd.Args) > 0 {
			name = strings.Join(cmd.Args, " ")
		}
		return lines("Hello, " + name + "!"), nil
	case "upper":
		if len(cmd.Args) > 0 {
			return lines(strings.ToUpper(
				strings.Join(cmd.Args, " "),
			)), nil
		}
		return lines("UPPER: provide text or pipe input"), nil
	case "rev":
		if len(cmd.Args) == 0 {
			return lines("rev: provide text"), nil
		}
		s := strings.Join(cmd.Args, " ")
		runes := []rune(s)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return lines(string(runes)), nil
	case "repeat":
		if len(cmd.Args) < 1 {
			return nil, errors.New("usage: repeat <text>")
		}
		text := strings.Join(cmd.Args, " ")
		var out []component.Responsive
		for i := range 3 {
			out = append(out, responsive(
				fmt.Sprintf("%d: %s", i+1, text),
			))
		}
		return iterator.FromSlice(out), nil
	case "date":
		return lines(time.Now().Format(time.RFC1123)), nil
	default:
		return nil, repl.ErrNotFound
	}
}

func (handler) Complete(
	_ context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if len(args) >= 1 {
		return iterator.Empty[string](), nil
	}
	var matches []string
	for _, c := range commands {
		if strings.HasPrefix(c, cmd) {
			matches = append(matches, c)
		}
	}
	return iterator.FromSlice(matches), nil
}

func responsive(s string) component.Responsive {
	return component.NewResponsiveString(
		s, component.StringResponsiveConfig{},
	)
}

func lines(ss ...string) iterator.Iterator[component.Responsive] {
	out := make([]component.Responsive, len(ss))
	for i, s := range ss {
		out[i] = responsive(s)
	}
	return iterator.FromSlice(out)
}

func main() {
	h := repl.New(
		sh.New(handler{}),
		term.ScheduleNextTick,
		term.FuncInterrupter(func(context.Context) error {
			term.ScheduleNextTick(func() {})
			return nil
		}),
		repl.WithPrompt("$ "),
		repl.WithTabStyle(inputbox.TabPrints),
	)
	if err := tui.Run(h); err != nil {
		log.Fatal(err)
	}
}
