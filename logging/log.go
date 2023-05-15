package logging

import (
	"os"
	"time"

	"github.com/ernestrc/blue/logging/trace"
	logd "github.com/ernestrc/logd-go/logging"
	log "github.com/sirupsen/logrus"
)

const (
	KeyThread        = logd.KeyThread
	KeyClass         = logd.KeyClass
	KeyLevel         = logd.KeyLevel
	KeyTime          = logd.KeyTime
	KeyDate          = logd.KeyDate
	KeyTimestamp     = logd.KeyTimestamp
	KeyMessage       = logd.KeyMessage
	KeyFunc          = "func"
	KeyFile          = "file"
	KeyCallType      = "callType"
	KeyStep          = "step"
	KeyTraceID       = "traceID"
	ValueStepSuccess = "success"
	ValueStepFailure = "failure"
	ValueStepAttempt = "attempt"
	KeyError         = "error"
)

// Field represents a key valye pair.
type Field struct {
	Key   string
	Value string
}

// LogAttempt logs a trace-level log with KeyStep set to Attempt and
// returns the current timestamp.
func LogAttempt(traceID trace.ID, callType string, extra ...Field) time.Time {
	fields := log.Fields{
		KeyCallType: callType,
		KeyStep:     ValueStepAttempt,
		KeyTraceID:  string(traceID),
	}
	for _, f := range extra {
		fields[f.Key] = f.Value
	}
	log.WithFields(fields).Trace()
	return time.Now()
}

// LogResultInfo logs a info-level log. See LogResultLevel for more details.
func LogResultInfo(
	err error, attemptAt time.Time,
	traceID trace.ID, callType string, extra ...Field,
) {
	LogResultLevel(log.InfoLevel, err, attemptAt, traceID, callType, extra...)
}

// LogResult logs a debug-level log. See LogResultLevel for more details.
func LogResult(
	err error, attemptAt time.Time,
	traceID trace.ID, callType string, extra ...Field,
) {
	LogResultLevel(log.DebugLevel, err, attemptAt, traceID, callType, extra...)
}

// LogResultLevel logs a log with KeyStep set to Success, if error
// is nil or logs an error-level log with KeyStep set to Failure if error
// is not nil. It uses attemptAt to calculate the duration between attempt
// and resolution.
func LogResultLevel(
	level log.Level, err error, attemptAt time.Time,
	traceID trace.ID, callType string, extra ...Field,
) {
	fields := log.Fields{
		KeyCallType:   callType,
		KeyTraceID:    string(traceID),
		"duration_us": MicrosecondsSince(attemptAt),
	}
	for _, f := range extra {
		fields[f.Key] = f.Value
	}
	if err == nil {
		fields[KeyStep] = ValueStepSuccess
		log.WithFields(fields).Log(level)
	} else {
		fields[KeyStep] = ValueStepFailure
		fields[KeyError] = err.Error()
		log.WithFields(fields).Error()
	}
}

// SetDefaults sets default level, formatter and output.
func SetDefaults(debug bool) {
	formatter := &LogrusFormatter{}
	if debug {
		log.SetLevel(log.TraceLevel)
		formatter.Debug = true
	} else {
		log.SetLevel(log.InfoLevel)
	}

	log.SetFormatter(formatter)
	log.SetOutput(os.Stdout)
}
