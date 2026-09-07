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

package main

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/cli"
)

const (
	testProjectID = "1234"
)

func skipIfServerNotRunning(t *testing.T, addr string) {
	conn, err := net.Dial("tcp", addr)
	if err == nil {
		_ = conn.Close()
		return
	}
	t.Logf("server not running on %s, skipping test '%s'", addr, t.Name())
	t.SkipNow()
}

func newTestCLI(t *testing.T) cli.CLI {
	tempDir, err := os.MkdirTemp("", "blue_test")
	require.NoError(t, err)

	i := newInitializer(tempDir)
	require.NoError(t, i.Initialize(testProjectID))

	c, err := newBlueCtl(tempDir)
	require.NoError(t, err)

	return c
}

func TestSourceGet(t *testing.T) {
	skipIfServerNotRunning(t, "127.0.0.1:4238")

	c := newTestCLI(t)
	ctx := context.Background()

	assert.NoError(t, c.Run(ctx, []string{"source", "get", "1235"}))
}
