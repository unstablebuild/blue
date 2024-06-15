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
package logging

import (
	"os"
	"time"

	"github.com/unstablebuild/blue/logging/trace"
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
	LogResultLevel(log.InfoLevel, log.ErrorLevel, err, attemptAt, traceID, callType, extra...)
}

// LogResultTrace logs a trace-level log in case of success, and debug-level log in case of error.
// See LogResultLevel for more details.
func LogResultTrace(
	err error, attemptAt time.Time,
	traceID trace.ID, callType string, extra ...Field,
) {
	LogResultLevel(log.TraceLevel, log.DebugLevel, err, attemptAt, traceID, callType, extra...)
}

// LogResult logs a debug-level log. See LogResultLevel for more details.
func LogResult(
	err error, attemptAt time.Time,
	traceID trace.ID, callType string, extra ...Field,
) {
	LogResultLevel(log.DebugLevel, log.ErrorLevel, err, attemptAt, traceID, callType, extra...)
}

// LogResultLevel logs a log with KeyStep set to Success, if error
// is nil or logs an error-level log with KeyStep set to Failure if error
// is not nil. It uses attemptAt to calculate the duration between attempt
// and resolution.
func LogResultLevel(
	level, errorLevel log.Level, err error, attemptAt time.Time,
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
		log.WithFields(fields).Log(errorLevel)
	}
}

// SetDefaults sets default level, formatter and output.
func SetDefaults(debug bool) {
	formatter := &LogrusLogdFormatter{}
	if debug {
		log.SetLevel(log.TraceLevel)
		formatter.Debug = true
	} else {
		log.SetLevel(log.InfoLevel)
	}

	log.SetFormatter(formatter)
	log.SetOutput(os.Stdout)
}
