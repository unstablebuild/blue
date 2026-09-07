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
