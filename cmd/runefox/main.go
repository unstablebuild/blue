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
	"fmt"
	"log/slog"
	"net/url"
	"os"

	htmlhandler "github.com/unstablebuild/blue/tui/handler/html"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: runefox <url>")
		os.Exit(1)
	}

	u, err := url.Parse(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "usage: runefox <url>")
		os.Exit(1)
	}

	interrupter := term.FuncInterrupter(func(context.Context) error {
		term.ScheduleNextTick(func() {})
		return nil
	})

	h := htmlhandler.New(interrupter, u)
	defer func() { _ = h.Close() }()

	if err := tui.Run(h, tui.WithInputMode(term.InputEsc|term.InputMouse)); err != nil {
		slog.Error("run error", "error", err)
	}
}
