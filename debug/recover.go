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

package debug

import (
	"fmt"
	"runtime/debug"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/issue"
)

const (
	reportMetadataErrorField      = "error"
	reportMetadataStackTraceField = "stack"
	reportMetadataPanicField      = "panic"
	reportMetadataBugField        = "bug"
)

// CapturePanic attempts to capture a panic during execution of f, logs it
// and returns a Report and false, or returns true if f returned
// successfully.
func CapturePanic(log *log.Logger, pkg, version string, f func()) (
	report issue.Report, panicValue any, ok bool,
) {
	defer func() {
		panicValue = recover()
		if panicValue == nil {
			return
		}
		report = BuildCrashReport(pkg, version, panicValue)

		log.Errorf("CapturePanic: panic: %v", report.Metadata[reportMetadataErrorField])
	}()

	f()
	ok = true
	return
}

// BuildCrashReport builds a crash report with the given panic value, for
// the given package and version.
func BuildCrashReport(pkg, version string, panicValue any) (
	report issue.Report,
) {
	report = issue.Report{
		Author:    "debug.CapturePanic",
		Package:   pkg,
		Version:   version,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}

	bi, ok := debug.ReadBuildInfo()
	if ok {
		report.Build = *bi
	}

	var errStr string
	switch x := panicValue.(type) {
	case string:
		errStr = x
	case error:
		errStr = x.Error()
	default:
		errStr = fmt.Sprintf("unknown: %v", panicValue)
	}
	report.Metadata[reportMetadataStackTraceField] = string(debug.Stack())
	report.Metadata[reportMetadataBugField] = ""
	report.Metadata[reportMetadataPanicField] = ""
	report.Subject = fmt.Sprintf("%20s", errStr)
	report.Metadata[reportMetadataErrorField] = errStr

	return
}
