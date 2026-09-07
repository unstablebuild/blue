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
