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
