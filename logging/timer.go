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
