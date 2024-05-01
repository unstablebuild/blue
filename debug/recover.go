package debug

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/unstablebuild/blue/issue"
	log "github.com/sirupsen/logrus"
)

const (
	ReportMetadataErrorField      = "error"
	ReportMetadataStackTraceField = "stack"
)

// CapturePanic attempts to capture a panic during execution of f, logs it
// and returns a Report and false, or returns true if f returned
// successfully.
func CapturePanic(log *log.Logger, pkg, version string, f func()) (ok bool, report issue.Report) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
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
		switch x := r.(type) {
		case string:
			errStr = x
		case error:
			errStr = x.Error()
		default:
			errStr = fmt.Sprintf("unknown: %v", r)
		}
		report.Metadata[ReportMetadataStackTraceField] = string(debug.Stack())
		report.Subject = fmt.Sprintf("%20s", errStr)
		report.Metadata[ReportMetadataErrorField] = errStr

		log.Errorf("CapturePanic: panic: %v", report.Metadata[ReportMetadataErrorField])
		ok = false
	}()

	f()
	ok = true
	return
}
