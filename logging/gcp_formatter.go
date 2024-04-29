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
	keyTimestamp      = "timestamp"
	keySourceLocation = "logging.googleapis.com/sourceLocation"
)

// LogrusGCPFormatter is a logrus.Formatter compatible with google cloud's structured logging.
type LogrusGCPFormatter struct {
}

// Format renders a single log entry in a format compatible with Google Cloud Logging.
func (f LogrusGCPFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	f.resetTopLevelFields(entry)
	out, err := json.Marshal(entry.Data)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}

	out = append(out, '\n')
	return out, nil
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
