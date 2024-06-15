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
	"encoding/json"
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
