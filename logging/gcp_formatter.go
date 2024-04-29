package logging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// LogrusGCPFormatter is a logrus.Formatter compatible with google cloud's structured logging.
type LogrusGCPFormatter struct {
}

// Format renders a single log entry in a format compatible with Google Cloud Logging.
func (f LogrusGCPFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var log gcpEntry
	f.setTopLevelFields(&log, entry)
	f.setCustomFields(&log, entry)

	out, err := json.Marshal(&log)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}

	return out, nil
}

type sourceLocation struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

type gcpEntry struct {
	Message        string         `json:"message"`
	Severity       string         `json:"severity,omitempty"`
	Timestamp      string         `json:"timestamp,omitempty"`
	SourceLocation sourceLocation `json:"logging.googleapis.com/sourceLocation,omitempty"`

	Payload map[string]any `json:"jsonPayload"`
}

func (f LogrusGCPFormatter) setTopLevelFields(log *gcpEntry, entry *logrus.Entry) {
	switch entry.Level {
	case logrus.TraceLevel, logrus.DebugLevel:
		log.Severity = "DEBUG"
	case logrus.InfoLevel:
		log.Severity = "INFO"
	case logrus.WarnLevel:
		log.Severity = "WARNING"
	case logrus.ErrorLevel:
		log.Severity = "ERROR"
	case logrus.PanicLevel:
		log.Severity = "CRITICAL"
	case logrus.FatalLevel:
		log.Severity = "EMERGENCY"
	}
	log.Timestamp = entry.Time.Format(time.RFC3339Nano)
	log.Message = entry.Message
	if entry.HasCaller() {
		log.SourceLocation.File = entry.Caller.File
		log.SourceLocation.Line = entry.Caller.Line
		log.SourceLocation.Function = entry.Caller.Function
	}
}

func (f LogrusGCPFormatter) setCustomFields(log *gcpEntry, entry *logrus.Entry) {
	log.Payload = entry.Data
}
