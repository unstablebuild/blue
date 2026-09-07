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
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	expectedLog1 = `{"time": "2017-09-07T14:54:39.474Z","severity":"DEBUG","thread":"pool-5-thread-6","class":"control.RaptorHandler","flow": "Publish", "step": "Attempt", "operation": "CreatePublisher", "logging.googleapis.com/sourceLocation":{"file":"shouldNotOverwrite.go","line":123}}`
	expectedLog2 = `{"time": "2017-09-07T14:54:39.474Z","severity":"DEBUG","thread":"2223","flow": null, "logging.googleapis.com/sourceLocation":{"file":"myFile.go","line":223}}`
	expectedLog3 = `{"time": "2017-09-07T14:54:39.474Z","severity":"DEBUG","thread":"2223","logging.googleapis.com/sourceLocation":{"file":"myFile.go","function": "myFunc","line":223}, "flow": null}`
	expectedLog4 = `{"time": "2017-09-07T14:54:39.474Z","severity":"DEBUG","callType":"myCallType","hello": "yeah"}`
)

func TestGCPFormatter(t *testing.T) {
	tsuite := []struct {
		input          logrus.Fields
		file           string
		fn             string
		line           int
		expectedOutput string
	}{
		{testCase1(), "shouldNotOverwrite.go", "", 123, expectedLog1},
		{testCase2(), "myFile.go", "", 223, expectedLog2},
		{testCase3(), "myFile.go", "myFunc", 223, expectedLog3},
		{testCase4(), "", "", 0, expectedLog4},
	}

	for i, tcase := range tsuite {
		f, entry := setupGCPTestCase(tcase.input, tcase.file, tcase.fn, tcase.line)

		log, err := f.Format(entry)
		assert.NoError(t, err)

		// parse output so we can test equality without worrying about
		// order of properties
		var actualOutput, expectedOutput map[string]any
		err = json.Unmarshal([]byte(tcase.expectedOutput), &expectedOutput)
		require.NoError(t, err, i)
		err = json.Unmarshal(log, &actualOutput)
		require.NoError(t, err, i)

		assert.Equal(t, expectedOutput, actualOutput, i)
	}
}

func BenchmarkGCPFormatterDebugOn(b *testing.B) {
	f, entry := setupGCPTestCase(makeBenchProps(),
		"myjfkewfjelfjlFile.go", "myGoFunc", 123345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.Format(entry)
	}
}

func BenchmarkGCPFormatterDebugOff(b *testing.B) {
	f, entry := setupGCPTestCase(makeBenchProps(),
		"myjfkewfjelfjlFile.go", "myGoFunc", 123345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.Format(entry)
	}
}

// TestGCPFormatterStringifiesErrors guards against the regression where
// log.WithError(err) rendered as "error": {} because json.Marshal encodes
// an error's unexported struct fields instead of its message.
func TestGCPFormatterStringifiesErrors(t *testing.T) {
	logger := logrus.New()
	f := LogrusGCPFormatter{}
	logger.SetFormatter(&f)

	wrapped := fmt.Errorf("sync failed: %w", errors.New("http response status: 400 Bad Request"))

	entry := logrus.NewEntry(logger).
		WithError(wrapped).
		WithField("cause", errors.New("secondary boom"))
	entry.Level = logrus.ErrorLevel
	entry.Message = "release handler failure"

	out, err := f.Format(entry)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(out, &parsed))

	assert.Equal(t, "sync failed: http response status: 400 Bad Request", parsed[logrus.ErrorKey])
	assert.Equal(t, "secondary boom", parsed["cause"])
}

func setupGCPTestCase(
	input logrus.Fields, file, fn string, line int,
) (f LogrusGCPFormatter, entry *logrus.Entry) {
	var err error
	logger := logrus.New()

	f = LogrusGCPFormatter{}

	logger.SetFormatter(&f)

	entry = logrus.NewEntry(logger)
	entry.Time, err = time.Parse("2006-01-02 15:04:05.999",
		input[KeyTimestamp].(string))
	if err != nil {
		panic(err)
	}
	delete(input, KeyTimestamp)
	entry = entry.WithFields(input)

	if file == "" && line == 0 {
		/* leave caller as nil so we test that */
	} else {
		entry.Caller = &runtime.Frame{}
		entry.Caller.Function = fn
		entry.Caller.File = file
		entry.Caller.Line = line
		entry.Logger.ReportCaller = true
	}

	entry.Level = logrus.DebugLevel
	return
}
