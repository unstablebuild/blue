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
