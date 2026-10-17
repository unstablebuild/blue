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
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term/progstatus"
)

// encodeReports returns the escape sequences bluectl writes for reports.
func encodeReports(reports ...progstatus.Report) string {
	var b strings.Builder
	for _, r := range reports {
		r.App = statusApp
		b.WriteString(r.Encode(false))
	}
	return b.String()
}

func TestRunReportsStatus(t *testing.T) {
	dir := t.TempDir()
	licenseFile := filepath.Join(dir, "LICENSE_HEADER")
	require.NoError(t, os.WriteFile(licenseFile,
		[]byte("// Copyright Test\n// Licensed under the Test License\n"), 0o600))
	missing := filepath.Join(dir, "missing")
	newGoFile := func(t *testing.T) string {
		f := filepath.Join(t.TempDir(), "a.go")
		require.NoError(t, os.WriteFile(f, []byte("package a\n"), 0o600))
		return f
	}

	tsuite := []struct {
		name    string
		args    func(t *testing.T) []string
		want    []progstatus.Report
		wantErr bool
	}{
		{
			name: "done with progress",
			args: func(t *testing.T) []string {
				return []string{"license", licenseFile, newGoFile(t), newGoFile(t)}
			},
			want: []progstatus.Report{
				{State: progstatus.Working, Msg: "license"},
				{State: progstatus.Working, Progress: progstatus.Percent(50),
					Msg: "Checking license headers of 2 files"},
				{State: progstatus.Working, Progress: progstatus.Percent(100),
					Msg: "Checking license headers of 2 files"},
				{State: progstatus.Done, Msg: "license"},
			},
		},
		{
			name: "error",
			args: func(t *testing.T) []string {
				return []string{"license", missing, newGoFile(t)}
			},
			want: []progstatus.Report{
				{State: progstatus.Working, Msg: "license"},
				{State: progstatus.Error, Msg: "license: license: read file: open " +
					missing + ": no such file or directory"},
			},
			wantErr: true,
		},
		{
			name: "unknown command",
			args: func(*testing.T) []string { return []string{"nope"} },
			want: []progstatus.Report{
				{State: progstatus.Error, Msg: "invalid usage of CLI"},
			},
			wantErr: true,
		},
		{
			name: "version",
			args: func(*testing.T) []string { return []string{"-v"} },
		},
		{
			name: "help",
			args: func(*testing.T) []string { return []string{"-h"} },
		},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			configFolder := t.TempDir()
			require.NoError(t, newInitializer(configFolder).Initialize(testProjectID))
			var out bytes.Buffer
			c, err := newBlueCtl(configFolder,
				progstatus.NewReporter(&out, statusApp))
			require.NoError(t, err)

			err = c.Run(context.Background(), tcase.args(t))

			if tcase.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, encodeReports(tcase.want...), out.String())
		})
	}
}

func TestInitializerReportsBlocked(t *testing.T) {
	var out bytes.Buffer
	status := progstatus.NewReporter(&out, statusApp)
	ctx := progstatus.NewContext(context.Background(), status)
	i := initializer{
		configFolder: t.TempDir(),
		inputReader:  strings.NewReader(testProjectID + "\n"),
		outputWriter: io.Discard,
	}
	status.Report(progstatus.Report{State: progstatus.Working, Msg: "issue"})

	require.NoError(t, i.Run(ctx, nil))

	require.Equal(t, encodeReports(
		progstatus.Report{State: progstatus.Working, Msg: "issue"},
		progstatus.Report{State: progstatus.Blocked, Kind: progstatus.Question,
			Msg: "Google Cloud project ID required to configure bluectl"},
		progstatus.Report{State: progstatus.Working, Msg: "issue"},
	), out.String())
}

func TestReportInterruptsHelperProcess(t *testing.T) {
	tty := os.Getenv("BLUECTL_STATUS_HELPER_TTY")
	if tty == "" {
		return
	}
	f, err := os.OpenFile(tty, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		os.Exit(2)
	}
	reportInterrupts(progstatus.NewReporter(f, statusApp))
	_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
	time.Sleep(5 * time.Second)
	os.Exit(0)
}

func TestReportInterrupts(t *testing.T) {
	tty := filepath.Join(t.TempDir(), "tty")
	require.NoError(t, os.WriteFile(tty, nil, 0o600))
	cmd := exec.Command(os.Args[0], "-test.run=^TestReportInterruptsHelperProcess$")
	cmd.Env = append(os.Environ(), "BLUECTL_STATUS_HELPER_TTY="+tty)

	err := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	ws, ok := exitErr.Sys().(syscall.WaitStatus)
	require.True(t, ok)
	require.True(t, ws.Signaled(), "bluectl dies of the signal: %v", err)
	require.Equal(t, syscall.SIGINT, ws.Signal())
	data, err := os.ReadFile(tty)
	require.NoError(t, err)
	require.Equal(t, encodeReports(progstatus.Report{
		State: progstatus.Idle, Msg: "Interrupted",
	}), string(data))
}
