package debug

import (
	"fmt"
	"runtime/debug"
	"time"

	log "github.com/sirupsen/logrus"
)

// PanicReport contains informatin about a panic
type PanicReport struct {
	Package   string
	Version   string
	Build     debug.BuildInfo
	Error     string
	Stack     string
	CreatedAt time.Time
	Metadata  map[string]string
}

// CapturePanic attempts to capture a panic during execution of f, logs it
// and returns a PanicReport and false, or returns true if f returned
// successfully.
func CapturePanic(log *log.Logger, pkg, version string, f func()) (ok bool, report PanicReport) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		report = PanicReport{
			Package:   pkg,
			Version:   version,
			Stack:     string(debug.Stack()),
			CreatedAt: time.Now(),
		}
		bi, ok := debug.ReadBuildInfo()
		if ok {
			report.Build = *bi
		}
		switch x := r.(type) {
		case string:
			report.Error = x
		case error:
			report.Error = x.Error()
		default:
			report.Error = fmt.Sprintf("unknown: %v", r)
		}

		log.Errorf("CapturePanic: panic: %v", report.Error)
		ok = false
	}()

	f()
	ok = true
	return
}
