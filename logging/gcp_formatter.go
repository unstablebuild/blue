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

package logging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	keySeverity       = "severity"
	keyMessage        = "message"
	keyTimestamp      = "time"
	keySourceLocation = "logging.googleapis.com/sourceLocation"
)

// LogrusGCPFormatter is a logrus.Formatter compatible with google cloud's structured logging.
type LogrusGCPFormatter struct {
}

// Format renders a single log entry in a format compatible with Google Cloud Logging.
func (f LogrusGCPFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	f.resetTopLevelFields(entry)
	stringifyErrors(entry.Data)
	out, err := json.Marshal(entry.Data)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}

	out = append(out, '\n')
	return out, nil
}

// stringifyErrors replaces error-typed field values with their Error()
// string. json.Marshal serializes an error interface value by encoding
// its (usually unexported) struct fields, which yields "{}" and drops the
// message entirely. Mirrors logrus.JSONFormatter, which special-cases
// errors the same way. Applies to every field, not just logrus.ErrorKey,
// so callers using WithField("cause", err) are covered too.
func stringifyErrors(data logrus.Fields) {
	for k, v := range data {
		if err, ok := v.(error); ok {
			data[k] = err.Error()
		}
	}
}

type sourceLocation struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

func (f LogrusGCPFormatter) resetTopLevelFields(entry *logrus.Entry) {
	switch entry.Level {
	case logrus.TraceLevel, logrus.DebugLevel:
		entry.Data[keySeverity] = "DEBUG"
	case logrus.InfoLevel:
		entry.Data[keySeverity] = "INFO"
	case logrus.WarnLevel:
		entry.Data[keySeverity] = "WARNING"
	case logrus.ErrorLevel:
		entry.Data[keySeverity] = "ERROR"
	case logrus.PanicLevel:
		entry.Data[keySeverity] = "CRITICAL"
	case logrus.FatalLevel:
		entry.Data[keySeverity] = "EMERGENCY"
	}
	entry.Data[keyTimestamp] = entry.Time.Format(time.RFC3339Nano)
	if entry.Message != "" {
		entry.Data[keyMessage] = entry.Message
	}
	if entry.HasCaller() {
		var loc sourceLocation
		loc.File = entry.Caller.File
		loc.Line = entry.Caller.Line
		loc.Function = entry.Caller.Function
		entry.Data[keySourceLocation] = loc
	}
}
