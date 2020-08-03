package logging

import (
	"net/http"
	"time"

	"github.com/ernestrc/blue/logging/trace"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// HeaderTraceID is an HTTP header key used to trace HTTP requests.
const HeaderTraceID = "X-Blue-Trace-ID"

// Middleware implements http.Handler by wrapping another handler
// and adding logging to it.
type Middleware struct {
	inner  http.Handler
	logger *log.Logger
}

// NewMiddleware returns an instance of Middleware.
func NewMiddleware(inner http.Handler) (m *Middleware) {
	m = new(Middleware)
	m.inner = inner
	m.logger = log.StandardLogger()
	return
}

// WithLogger returns an instance of Middleware with the given logger.
func (m *Middleware) WithLogger(logger *log.Logger) *Middleware {
	m.logger = logger
	return m
}

type responseWriter struct {
	inner   http.ResponseWriter
	status  int
	resSize int
}

func (w *responseWriter) Write(data []byte) (n int, err error) {
	n, err = w.inner.Write(data)
	w.resSize += n
	return
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.inner.WriteHeader(statusCode)
	w.status = statusCode
}

func (w *responseWriter) Header() http.Header {
	return w.inner.Header()
}

func getTraceID(r *http.Request) string {
	if traceID := r.Header.Get(HeaderTraceID); traceID != "" {
		return traceID
	}
	traceID := uuid.New()
	return traceID.String()
}

func setTraceID(traceID string, r *http.Request) *http.Request {
	ctx := trace.NewContext(r.Context(), trace.ID(traceID))
	r = r.WithContext(ctx)
	return r
}

func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	writer := responseWriter{inner: w}

	traceID := getTraceID(r)
	r = setTraceID(traceID, r)

	fields := log.Fields{
		KeyCallType:  "ServeHTTP",
		KeyStep:      ValueStepAttempt,
		KeyTraceID:   traceID,
		"method":     r.Method,
		"uri":        r.RequestURI,
		"remote":     r.RemoteAddr,
		"referer":    r.Referer(),
		"user-agent": r.UserAgent(),
	}

	m.logger.WithFields(fields).Debug()

	start := time.Now()
	m.inner.ServeHTTP(&writer, r)

	status := writer.status
	if status == 0 {
		status = 200
	}

	fields["status"] = status
	fields["duration_us"] = MicrosecondsSince(start)
	fields["size"] = writer.resSize

	if status >= 500 {
		fields[KeyStep] = ValueStepFailure
		m.logger.WithFields(fields).Error()
		return
	}

	fields[KeyStep] = ValueStepSuccess

	if status >= 300 {
		m.logger.WithFields(fields).Warning()
		return
	}
	m.logger.WithFields(fields).Info()
}
