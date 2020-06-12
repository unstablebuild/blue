package logging

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

type Timer struct {
	ctx   context.Context
	start time.Time
	name  string
}

type TimerWithLogger struct {
	Timer
	logger *logrus.Logger
}

func MicrosecondsSince(start time.Time) int64 {
	return int64(float64(time.Since(start).Nanoseconds()) / float64(1000))
}

func NewTimer(ctx context.Context, name string) Timer {
	return Timer{ctx, time.Now(), name}
}

func (t Timer) WithLogger(logger *logrus.Logger) TimerWithLogger {
	return TimerWithLogger{Timer: t, logger: logger}
}

func (t Timer) LogDuration() {
	t.WithLogger(logrus.StandardLogger()).LogDuration()
}

func (t TimerWithLogger) LogDuration() {
	t.logger.WithFields(logrus.Fields{
		KeyCallType:   t.name,
		"duration_us": MicrosecondsSince(t.start),
	}).Info()
}
