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

package ipblock

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/ipblock"
)

// run runs the bluectl ipblock command over blocklist and returns what
// it printed.
func run(t *testing.T, blocklist ipblock.List, now time.Time, args ...string) (string, error) {
	t.Helper()
	command := NewCLI(blocklist, "ops").(*ipblockCLI)
	out := new(bytes.Buffer)
	command.cmds[actionAdd].(*addCLI).out = out
	command.cmds[actionAdd].(*addCLI).now = func() time.Time { return now }
	command.cmds[actionRemove].(*removeCLI).out = out
	command.cmds[actionList].(*listCLI).out = out
	err := command.Run(context.Background(), args)
	return out.String(), err
}

func listed(t *testing.T, blocklist ipblock.List) map[string]ipblock.Block {
	t.Helper()
	blocks, err := blocklist.List(context.Background())
	require.NoError(t, err)
	ret := make(map[string]ipblock.Block, len(blocks))
	for _, b := range blocks {
		ret[b.Prefix.String()] = b
	}
	return ret
}

func TestAdd(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	blocklist := ipblock.NewMemoryList()

	out, err := run(t, blocklist, now, "add", "-r", "spam", "-t", "24h",
		"203.0.113.7", "198.51.100.7/24", "::ffff:192.0.2.1", "2001:db8::1/64")
	require.NoError(t, err)
	until := now.Add(24 * time.Hour).Format(time.RFC3339)
	assert.Equal(t, "blocked 203.0.113.7/32 until "+until+"\n"+
		"blocked 198.51.100.0/24 until "+until+"\n"+
		"blocked 192.0.2.1/32 until "+until+"\n"+
		"blocked 2001:db8::/64 until "+until+"\n", out)

	blocks := listed(t, blocklist)
	require.Len(t, blocks, 4)
	b := blocks["198.51.100.0/24"]
	assert.Equal(t, "spam", b.Reason)
	assert.Equal(t, "ops", b.BlockedBy)
	assert.True(t, now.Equal(b.BlockedAt))
	assert.True(t, now.Add(24*time.Hour).Equal(b.ExpiresAt))

	out, err = run(t, blocklist, now, "add", "203.0.113.7")
	require.NoError(t, err)
	assert.Equal(t, "blocked 203.0.113.7/32 until removed\n", out)
	assert.True(t, listed(t, blocklist)["203.0.113.7/32"].ExpiresAt.IsZero(),
		"blocking again replaces the block")
}

func TestAdd_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no address", args: []string{"add"}},
		{name: "not an address", args: []string{"add", "203.0.113.7", "bogus"}, wantErr: "neither an address nor a prefix"},
		{name: "bad prefix", args: []string{"add", "203.0.113.7/33"}},
		// cli.Parse counts a dash-led value as a flag, so a negative -t
		// reaches add only when another flag follows it.
		{name: "negative TTL", args: []string{"add", "-t", "-1h", "-r", "x", "203.0.113.7"}, wantErr: "negative"},
		{name: "broad IPv4", args: []string{"add", "203.0.113.7", "10.0.0.0/15"}, wantErr: "pass -f"},
		{name: "broad IPv6", args: []string{"add", "2001:db8::/31"}, wantErr: "pass -f"},
		{name: "broad mapped IPv4", args: []string{"add", "::ffff:10.0.0.0/104"}, wantErr: "pass -f"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocklist := ipblock.NewMemoryList()
			_, err := run(t, blocklist, time.Now(), tc.args...)
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.wantErr)
			assert.Empty(t, listed(t, blocklist), "nothing is blocked when any argument is refused")
		})
	}
}

func TestAdd_ForceAllowsBroadPrefixes(t *testing.T) {
	blocklist := ipblock.NewMemoryList()
	_, err := run(t, blocklist, time.Now(), "add", "-f", "10.0.0.0/8", "2001:db8::/16")
	require.NoError(t, err)
	assert.Len(t, listed(t, blocklist), 2)
}

func TestRemove(t *testing.T) {
	ctx := context.Background()
	blocklist := ipblock.NewMemoryList()
	for _, p := range []string{"203.0.113.7/32", "198.51.100.0/24", "10.0.0.0/8"} {
		require.NoError(t, blocklist.Block(ctx, ipblock.Block{Prefix: netip.MustParsePrefix(p)}))
	}

	out, err := run(t, blocklist, time.Now(), "remove", "203.0.113.7", "192.0.2.1", "198.51.100.9/24")

	require.Error(t, err)
	assert.ErrorContains(t, err, "192.0.2.1/32 is not blocked")
	assert.Equal(t, "unblocked 203.0.113.7/32\nunblocked 198.51.100.0/24\n", out,
		"a prefix that is not blocked does not stop the others")
	assert.Equal(t, []string{"10.0.0.0/8"}, keys(listed(t, blocklist)))

	_, err = run(t, blocklist, time.Now(), "remove", "10.1.2.3")
	assert.ErrorContains(t, err, "not blocked", "only the exact prefix is lifted")
	_, err = run(t, blocklist, time.Now(), "remove", "bogus")
	assert.ErrorContains(t, err, "neither an address nor a prefix")
}

func keys(m map[string]ipblock.Block) []string {
	ret := make([]string, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}
	return ret
}

func TestList(t *testing.T) {
	ctx := context.Background()
	blocklist := ipblock.NewMemoryList()
	blockedAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	require.NoError(t, blocklist.Block(ctx, ipblock.Block{
		Prefix: netip.MustParsePrefix("203.0.113.0/24"), Reason: "spam", BlockedBy: "ops",
		BlockedAt: blockedAt, ExpiresAt: expiresAt,
	}))
	require.NoError(t, blocklist.Block(ctx, ipblock.Block{
		Prefix: netip.MustParsePrefix("198.51.100.7/32"), BlockedAt: blockedAt,
	}))

	t.Run("table", func(t *testing.T) {
		out, err := run(t, blocklist, time.Now(), "list")
		require.NoError(t, err)
		assert.Equal(t, [][]string{
			{"PREFIX", "EXPIRES", "BLOCKED BY", "BLOCKED AT", "REASON"},
			{"198.51.100.7/32", "never", "", blockedAt.Format(time.RFC3339), ""},
			{"203.0.113.0/24", expiresAt.Format(time.RFC3339), "ops", blockedAt.Format(time.RFC3339), "spam"},
		}, tableCells(out))
	})

	t.Run("json", func(t *testing.T) {
		out, err := run(t, blocklist, time.Now(), "list", "-F", "json")
		require.NoError(t, err)
		dec := json.NewDecoder(strings.NewReader(out))
		var got []map[string]any
		for dec.More() {
			var b map[string]any
			require.NoError(t, dec.Decode(&b))
			got = append(got, b)
		}
		require.Len(t, got, 2)
		assert.Equal(t, "198.51.100.7/32", got[0]["Prefix"])
		assert.NotContains(t, got[0], "ExpiresAt", "a block kept until removed has no expiry")
		assert.Equal(t, expiresAt.Format(time.RFC3339), got[1]["ExpiresAt"])
	})

	t.Run("template", func(t *testing.T) {
		out, err := run(t, blocklist, time.Now(), "list", "-F", "{{.Prefix}} {{.Reason}}")
		require.NoError(t, err)
		assert.Equal(t, "198.51.100.7/32 \n203.0.113.0/24 spam\n", out)
	})

	t.Run("rejects positional arguments", func(t *testing.T) {
		_, err := run(t, blocklist, time.Now(), "list", "extra")
		assert.Error(t, err)
	})
}

func TestCommandsRequireABlocklist(t *testing.T) {
	for _, args := range [][]string{
		{"add", "203.0.113.7"},
		{"remove", "203.0.113.7"},
		{"list"},
	} {
		_, err := run(t, nil, time.Now(), args...)
		assert.ErrorIs(t, err, errNotConfigured, args)
	}
}

// tableCells returns the trimmed cells of each row of a table printed
// by cliformat.Table, header first.
func tableCells(out string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "│") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "│"), "│")
		for i, cell := range cells {
			cells[i] = strings.TrimSpace(cell)
		}
		rows = append(rows, cells)
	}
	return rows
}
