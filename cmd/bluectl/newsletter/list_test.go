// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
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

package newsletter

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
)

func newTestList(t *testing.T, subscribers ...Subscriber) (*listCLI, *bytes.Buffer) {
	t.Helper()
	db := document.NewInMemoryService()
	t.Cleanup(func() { _ = db.Close() })
	for i, subscriber := range subscribers {
		require.NoError(t, db.Create(context.Background(),
			string(rune('a'+i)), subscriber))
	}
	command := newListCLI(db).(*listCLI)
	out := new(bytes.Buffer)
	command.out = out
	return command, out
}

func sortedLines(out *bytes.Buffer) []string {
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	sort.Strings(lines)
	return lines
}

func TestListEmailFormatPrintsOneAddressPerLine(t *testing.T) {
	command, out := newTestList(t,
		Subscriber{Email: "one@example.com", IP: "10.0.0.1"},
		Subscriber{Email: "two@example.com", IP: "10.0.0.2"})

	require.NoError(t, command.Run(context.Background(), []string{"-F", "email"}))
	assert.Equal(t, []string{"one@example.com", "two@example.com"}, sortedLines(out))
}

func TestListTemplateFormat(t *testing.T) {
	command, out := newTestList(t, Subscriber{Email: "one@example.com", IP: "10.0.0.1"})

	require.NoError(t, command.Run(context.Background(),
		[]string{"-F", "{{.Email}} {{.IP}}"}))
	assert.Equal(t, "one@example.com 10.0.0.1\n", out.String())
}

func TestListJSONFormat(t *testing.T) {
	subscribedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	command, out := newTestList(t,
		Subscriber{Email: "one@example.com", IP: "10.0.0.1", SubscribedAt: subscribedAt})

	require.NoError(t, command.Run(context.Background(), []string{"-F", "json"}))
	var got Subscriber
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, "one@example.com", got.Email)
	assert.Equal(t, "10.0.0.1", got.IP)
	assert.True(t, subscribedAt.Equal(got.SubscribedAt))
}

func TestListRejectsPositionalArguments(t *testing.T) {
	command, _ := newTestList(t)

	assert.Error(t, command.Run(context.Background(), []string{"extra"}))
}

func TestListRejectsInvalidSince(t *testing.T) {
	command, _ := newTestList(t)

	assert.Error(t, command.Run(context.Background(), []string{"-s", "yesterday"}))
}
