// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func TestIsTestFunc(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"TestAdd", true},
		{"TestAdd_subtraction", true},
		{"Test", true},
		{"BenchmarkAdd", true},
		{"Benchmark", true},
		{"FuzzParse", true},
		{"Fuzz", true},
		{"ExampleAdd", true},
		{"Example", true},
		{"main", false},
		{"init", false},
		{"helper", false},
		{"testHelper", false},
		{"benchmarkHelper", false},
		{"Testing", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTestFunc(tt.name))
		})
	}
}

func TestCompleteTestFuncs(t *testing.T) {
	results := []syntaxapi.Result{
		{Text: "TestAdd"},
		{Text: "main"},
		{Text: "BenchmarkSort"},
		{Text: "init"},
		{Text: "helper"},
		{Text: "FuzzParse"},
		{Text: "ExampleNew"},
	}
	parser := &stubParser{results: results}

	iter, err := completeTestFuncs(context.Background(), parser)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	var got []string
	for {
		s, ok := iter.Next(context.Background())
		if !ok {
			break
		}
		got = append(got, s)
	}
	require.NoError(t, iter.Err())
	assert.Equal(t, []string{"TestAdd", "BenchmarkSort", "FuzzParse", "ExampleNew"}, got)
}

type stubParser struct {
	syntaxapi.Parser
	results []syntaxapi.Result
}

func (p *stubParser) Search(
	_ string, _ []string, _ ...string,
) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice(p.results), nil
}

func TestProcessTestOutput(t *testing.T) {
	t.Run("all pass", func(t *testing.T) {
		mn := &mockNotifications{}
		h := &testCmd{notify: mn}

		jsonLines := strings.Join([]string{
			`{"Action":"run","Package":"example.com/test","Test":"TestAdd"}`,
			`{"Action":"output","Package":"example.com/test","Test":"TestAdd","Output":"=== RUN   TestAdd\n"}`,
			`{"Action":"run","Package":"example.com/test","Test":"TestAdd/case1"}`,
			`{"Action":"pass","Package":"example.com/test","Test":"TestAdd/case1","Elapsed":0.01}`,
			`{"Action":"pass","Package":"example.com/test","Test":"TestAdd","Elapsed":0.02}`,
			`{"Action":"pass","Package":"example.com/test","Elapsed":0.5}`,
		}, "\n")

		pr, pw := io.Pipe()
		go func() {
			_, _ = pw.Write([]byte(jsonLines))
			_ = pw.Close()
		}()

		exitErrCh := make(chan error, 1)
		exitErrCh <- nil

		var stderr bytes.Buffer
		h.processTestOutput("TestAdd", pr, &stderr, exitErrCh)

		msgs := mn.getMessages()
		require.GreaterOrEqual(t, len(msgs), 2)
		// First is the progress notification.
		assert.Equal(t, browserapi.LevelInfo, msgs[0].Level)
		assert.Contains(t, msgs[0].Message, "Running TestAdd")
		// Last is the success result.
		last := msgs[len(msgs)-1]
		assert.Equal(t, browserapi.LevelSuccess, last.Level)
		assert.Contains(t, last.Message, "passed")
	})

	t.Run("some fail", func(t *testing.T) {
		mn := &mockNotifications{}
		h := &testCmd{notify: mn}

		jsonLines := strings.Join([]string{
			`{"Action":"run","Package":"example.com/test","Test":"TestAdd"}`,
			`{"Action":"fail","Package":"example.com/test","Test":"TestAdd","Elapsed":0.01}`,
			`{"Action":"fail","Package":"example.com/test","Elapsed":0.5}`,
		}, "\n")

		pr, pw := io.Pipe()
		go func() {
			_, _ = pw.Write([]byte(jsonLines))
			_ = pw.Close()
		}()

		exitErrCh := make(chan error, 1)
		exitErrCh <- fmt.Errorf("exit status 1")

		var stderr bytes.Buffer
		h.processTestOutput("TestAdd", pr, &stderr, exitErrCh)

		msgs := mn.getMessages()
		require.GreaterOrEqual(t, len(msgs), 2)
		last := msgs[len(msgs)-1]
		assert.Equal(t, browserapi.LevelError, last.Level)
		assert.Contains(t, last.Message, "failed")
	})

	t.Run("build error", func(t *testing.T) {
		mn := &mockNotifications{}
		h := &testCmd{notify: mn}

		pr, pw := io.Pipe()
		go func() {
			_ = pw.Close()
		}()

		exitErrCh := make(chan error, 1)
		exitErrCh <- fmt.Errorf("exit status 2")

		var stderr bytes.Buffer
		stderr.WriteString("main_test.go:5:2: undefined: Add\n")
		h.processTestOutput("TestAdd", pr, &stderr, exitErrCh)

		msgs := mn.getMessages()
		require.GreaterOrEqual(t, len(msgs), 2)
		last := msgs[len(msgs)-1]
		assert.Equal(t, browserapi.LevelError, last.Level)
		assert.Contains(t, last.Message, "undefined: Add")
	})

	t.Run("no tests found", func(t *testing.T) {
		mn := &mockNotifications{}
		h := &testCmd{notify: mn}

		// go test -json outputs a pass with no Test field when no tests match.
		jsonLines := `{"Action":"pass","Package":"example.com/test","Elapsed":0.01}`

		pr, pw := io.Pipe()
		go func() {
			_, _ = pw.Write([]byte(jsonLines))
			_ = pw.Close()
		}()

		exitErrCh := make(chan error, 1)
		exitErrCh <- nil

		var stderr bytes.Buffer
		h.processTestOutput("TestNonexistent", pr, &stderr, exitErrCh)

		msgs := mn.getMessages()
		require.GreaterOrEqual(t, len(msgs), 2)
		last := msgs[len(msgs)-1]
		// No tests ran but process exited cleanly.
		assert.Equal(t, browserapi.LevelSuccess, last.Level)
		assert.Contains(t, last.Message, "passed")
	})
}
