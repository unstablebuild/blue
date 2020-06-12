package logging

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	logd "github.com/ernestrc/logd-go/logging"
	"github.com/sirupsen/logrus"
)

const (
	dateFormat = "2006-01-02"
	timeFormat = "15:04:05.999"
)

const (
	formatterWarningTypeKey      = "FORMATTER_WARNING_TYPE"
	formatterWarningTypeTemplate = "inefficient use of logrus formatter with key '%s'"

	formatterWarningCollisionKey      = "FORMATTER_WARNING_COLLISION"
	formatterWarningCollisionTemplate = "following keys are being overwritten by logging formatter %+v"
)

// LogrusFormatter implements logrus.Formatter interface with a log format
// that can be parsed by a logd server.
type LogrusFormatter struct {
	Debug bool
}

func (f *LogrusFormatter) setFields(log *logd.Log, entry *logrus.Entry) {
	for k, v := range entry.Data {
		switch v := v.(type) {
		case int:
			log.Set(k, strconv.FormatInt(int64(v), 10))
		case int8:
			log.Set(k, strconv.FormatInt(int64(v), 10))
		case int16:
			log.Set(k, strconv.FormatInt(int64(v), 10))
		case int32:
			log.Set(k, strconv.FormatInt(int64(v), 10))
		case int64:
			log.Set(k, strconv.FormatInt(v, 10))
		case uint:
			log.Set(k, strconv.FormatUint(uint64(v), 10))
		case uint8:
			log.Set(k, strconv.FormatUint(uint64(v), 10))
		case uint16:
			log.Set(k, strconv.FormatUint(uint64(v), 10))
		case uint32:
			log.Set(k, strconv.FormatUint(uint64(v), 10))
		case uint64:
			log.Set(k, strconv.FormatUint(v, 10))
		case float64:
			log.Set(k, strconv.FormatFloat(v, 'f', 3, 64))
		case float32:
			log.Set(k, strconv.FormatFloat(float64(v), 'f', 3, 32))
		case string:
			log.Set(k, v)
		case error:
			log.Set(k, v.Error())
		case bool:
			log.Set(k, strconv.FormatBool(v))
		default:
			log.Set(k, fmt.Sprintf("%+v", v))
			if f.Debug {
				log.Set(formatterWarningTypeKey,
					fmt.Sprintf(formatterWarningTypeTemplate, k))
			}
		}
	}
}

func getGID() string {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	return string(b)
}

func (f *LogrusFormatter) setHeader(log *logd.Log, entry *logrus.Entry) {
	log.Set(KeyDate, entry.Time.Format(dateFormat))
	log.Set(KeyTime, entry.Time.Format(timeFormat))
	log.Set(KeyMessage, entry.Message)
	log.Set(KeyLevel, strings.ToUpper(entry.Level.String()))
	if entry.HasCaller() {
		log.Set(KeyClass, entry.Caller.File)
	}
}

func detectCollisions(log *logd.Log, entry *logrus.Entry) {
	var collisionKeys []string
	for k := range entry.Data {
		switch k {
		case KeyThread, KeyDate, KeyTime, KeyMessage, KeyLevel, KeyFunc, KeyClass:
			collisionKeys = append(collisionKeys, k)
		}
	}
	if len(collisionKeys) != 0 {
		log.Set(formatterWarningCollisionKey,
			fmt.Sprintf(formatterWarningCollisionTemplate, collisionKeys))
	}
}

func (f *LogrusFormatter) setDebugFields(log *logd.Log, entry *logrus.Entry) {
	log.Set(KeyThread, getGID())
	if entry.HasCaller() {
		funcVal := fmt.Sprintf("%s.%d", entry.Caller.Function, entry.Caller.Line)
		log.Set(KeyFunc, funcVal)
	}
	detectCollisions(log, entry)
}

// Format renders a single log entry in a logd compatible format
func (f *LogrusFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var log logd.Log
	f.setFields(&log, entry)
	f.setHeader(&log, entry)
	if f.Debug {
		f.setDebugFields(&log, entry)
	}

	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = new(bytes.Buffer)
	}

	log.WriteTo(b)
	b.WriteRune('\n')
	return b.Bytes(), nil
}
