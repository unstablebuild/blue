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

package main

import (
	"io"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/term/progstatus"
	"golang.org/x/term"
)

// statusApp identifies bluectl in its program status reports.
const statusApp = "bluectl"

// newStatusReporter returns the reporter of bluectl's program status
// (OSC 7501). Reports go to stderr when it is a terminal, since stdout
// often carries data, else to the controlling terminal. It returns nil,
// which discards reports, when bluectl runs without a terminal.
func newStatusReporter() *progstatus.Reporter {
	var w io.Writer = os.Stderr
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		// Left open until bluectl exits.
		tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
		if err != nil {
			return nil
		}
		w = tty
	}
	return progstatus.NewReporter(w, statusApp)
}

// reportInterrupts reports idle when the user interrupts bluectl, as the
// protocol asks of a cancelled program, then lets the signal terminate
// bluectl as it would have.
func reportInterrupts(status *progstatus.Reporter) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		status.Report(progstatus.Report{
			State: progstatus.Idle, Msg: "Interrupted",
		})
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
		// Windows cannot send a process a signal, so exit there instead.
		p, err := os.FindProcess(os.Getpid())
		if err != nil || p.Signal(sig) != nil {
			os.Exit(1)
		}
	}()
}

// reportExit leaves a done or error record behind for the user to find,
// since bluectl exits as soon as command finishes.
func reportExit(status *progstatus.Reporter, command string, err error) {
	if err == nil {
		status.Report(progstatus.Report{State: progstatus.Done, Msg: command})
	} else {
		msg := err.Error()
		if command != "" {
			msg = command + ": " + msg
		}
		status.Report(progstatus.Report{State: progstatus.Error, Msg: msg})
	}
	if err := status.Err(); err != nil {
		log.Debugf("report program status: %v", err)
	}
}
