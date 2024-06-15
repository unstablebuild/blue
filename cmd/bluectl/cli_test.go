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
		conn.Close()
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
