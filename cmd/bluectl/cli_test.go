package main

import (
	"context"
	"io/ioutil"
	"net"
	"testing"

	"github.com/ernestrc/blue/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testProjectID = "1234"
)

func skipIfServerNotRunning(t *testing.T, addr string) {
	conn, err := net.Dial("tcp", addr)
	if err == nil {
		conn.Close()
		return
	}
	t.Logf("server not running on %s, skipping test '%s'", addr, t.Name())
	t.SkipNow()
}

func newTestCLI(t *testing.T) cli.CLI {
	tempDir, err := ioutil.TempDir("", "blue_test")
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
