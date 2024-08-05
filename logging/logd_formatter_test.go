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
	"fmt"
	"runtime"
	"testing"
	"time"

	logd "github.com/ernestrc/logd-go/logging"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	log1 = "2017-09-07 14:54:39.474	DEBUG	[pool-5-thread-6]	control.RaptorHandler	flow: Publish, step: Attempt, operation: CreatePublisher\n"
	log2 = "2017-09-07 14:54:39.474	DEBUG	[2223]	myFile.go	flow: <nil>\n"
	log3 = "2017-09-07 14:54:39.474	DEBUG	[9999999]	myFile.go	func: myFunc.223, flow: <nil>, FORMATTER_WARNING_TYPE: inefficient use of logrus formatter with key 'flow', FORMATTER_WARNING_COLLISION: following keys are being overwritten by logging formatter [thread]\n"
	log4 = "2017-09-07 14:54:39.474	DEBUG	[-]	-	myCallType: hello: yeah\n"
)

func testCase1() logrus.Fields {
	return logrus.Fields{
		KeyTimestamp: "2017-09-07 14:54:39.474",
		KeyThread:    "pool-5-thread-6",
		KeyClass:     "control.RaptorHandler",
		"flow":       "Publish",
		"step":       "Attempt",
		"operation":  "CreatePublisher",
	}
}

func testCase2() logrus.Fields {
	return logrus.Fields{
		KeyTimestamp: "2017-09-07 14:54:39.474",
		KeyThread:    "2223",
		"flow":       nil,
	}
}

func testCase3() logrus.Fields {
	return logrus.Fields{
		KeyTimestamp: "2017-09-07 14:54:39.474",
		KeyThread:    "2223",
		"flow":       nil,
	}
}

func testCase4() logrus.Fields {
	return logrus.Fields{
		KeyTimestamp: "2017-09-07 14:54:39.474",
		"callType":   "myCallType",
		"hello":      "yeah",
	}
}

func makeBenchProps() logrus.Fields {
	return logrus.Fields{
		KeyTimestamp: "2017-09-07 14:54:39.474",
		KeyThread:    "pool-5-thread-6",
		KeyClass:     "control.RaptorHandler",
		"flow":       "Publish",
		"step":       "Attempt",
		"operation":  "CreatePublisher",
	}
}

func TestLogdFormatter(t *testing.T) {
	tsuite := []struct {
		debug  bool
		input  logrus.Fields
		file   string
		fn     string
		line   int
		output string
	}{
		{false, testCase1(), "shouldNotOverwrite.go", "", 123, log1},
		{false, testCase2(), "myFile.go", "", 223, log2},
		{true, testCase3(), "myFile.go", "myFunc", 223, log3},
		{true, testCase4(), "", "", 0, log4},
	}

	for i, tcase := range tsuite {
		f, entry := setupLogdTestCase(tcase.debug,
			tcase.input, tcase.file, tcase.fn, tcase.line)

		log, err := f.Format(entry)
		assert.NoError(t, err)

		// parse output so we can test equality without worrying about
		// order of properties
		expectedOutput := &(logd.Parse(tcase.output)[0])
		testOutput := &(logd.Parse(string(log))[0])

		// assert props
		assert.ElementsMatch(t, expectedOutput.Props(), testOutput.Props(),
			fmt.Sprintf("test case %d not equal props: expected: %+v vs actual: %+v: %s", i,
				expectedOutput.Props(), testOutput.Props(), log))

		// assert header
		assertEqualProperty(t, i, KeyTimestamp, expectedOutput, testOutput)
		assertEqualProperty(t, i, KeyMessage, expectedOutput, testOutput)
		assertEqualProperty(t, i, KeyLevel, expectedOutput, testOutput)
		assertEqualProperty(t, i, KeyClass, expectedOutput, testOutput)
		assertEqualProperty(t, i, KeyTime, expectedOutput, testOutput)
		assertEqualProperty(t, i, KeyDate, expectedOutput, testOutput)

		// in debug mode goroutine ID is automatically added as log thread
		// but we don't have a way to stub that, so skip verification
		if !tcase.debug {
			assertEqualProperty(t, i, KeyThread, expectedOutput, testOutput)
		}
	}
}

func BenchmarkFormatterDebugOn(b *testing.B) {
	f, entry := setupLogdTestCase(true, makeBenchProps(),
		"myjfkewfjelfjlFile.go", "myGoFunc", 123345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.Format(entry)
	}
}

func BenchmarkFormatterDebugOff(b *testing.B) {
	f, entry := setupLogdTestCase(false, makeBenchProps(),
		"myjfkewfjelfjlFile.go", "myGoFunc", 123345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.Format(entry)
	}
}

func assertEqualProperty(
	t *testing.T, testCase int, key string,
	expectedOutput *logd.Log, testOutput *logd.Log,
) {
	expected, _ := expectedOutput.Get(key)
	actual, _ := testOutput.Get(key)
	assert.Equal(t, expected, actual,
		"property %s in log not equal in test case %d", key, testCase)
}

func setupLogdTestCase(
	debugMode bool, input logrus.Fields, file, fn string, line int,
) (f LogrusLogdFormatter, entry *logrus.Entry) {
	var err error
	logger := logrus.New()

	f = LogrusLogdFormatter{Debug: debugMode}

	logger.SetFormatter(&f)
	entry = logrus.NewEntry(logger).WithFields(input)

	entry.Time, err = time.Parse("2006-01-02 15:04:05.999",
		input[KeyTimestamp].(string))
	if err != nil {
		panic(err)
	}
	delete(input, KeyTimestamp)

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
