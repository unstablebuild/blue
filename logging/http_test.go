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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

type mockHandler struct {
	data    []byte
	latency time.Duration
}

func (m mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	time.Sleep(m.latency)
	_, _ = w.Write(m.data)
}

func TestMiddleware(t *testing.T) {
	// fixture
	fixData := []byte("la vida podria ser mes dura")
	fixMethod := "GET"
	fixLatency := 1 * time.Second
	fixTraceID := "fjelwkfjkewl"

	req := httptest.NewRequest(fixMethod, "http://127.0.0.1:8080/something", nil)

	// mocks
	logger, hook := logtest.NewNullLogger()
	logger.SetLevel(log.DebugLevel)
	mockHandler := mockHandler{data: fixData, latency: fixLatency}
	mockWriter := httptest.NewRecorder()
	req.Header.Set("X-Blue-Trace-ID", fixTraceID)

	// SUT
	m := NewMiddleware(mockHandler).WithLogger(logger)
	m.ServeHTTP(mockWriter, req)

	// verify
	if !assert.Len(t, hook.Entries, 2) {
		return
	}
	entry := hook.LastEntry()
	assert.Equal(t, log.InfoLevel, entry.Level)
	assert.Equal(t, 200, entry.Data["status"])
	assert.Equal(t, fixMethod, entry.Data["method"])
	assert.Equal(t, len(fixData), entry.Data["size"])
	assert.Equal(t, fixTraceID, entry.Data["traceID"])

	duration := time.Duration(entry.Data["duration_us"].(int64)) * time.Microsecond
	assert.Equal(t, int64(fixLatency.Seconds()), int64(duration.Seconds()))
}
