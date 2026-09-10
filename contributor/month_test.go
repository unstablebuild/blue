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

package contributor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonth(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		m, err := ParseMonth("2026-01")
		require.NoError(t, err)
		assert.Equal(t, Month("2026-01"), m)

		for _, bad := range []string{
			"", "2026", "2026-13", "2026-00", "2026-1", "01-2026",
			"2026-01-01", "garbage",
		} {
			_, err := ParseMonth(bad)
			assert.Error(t, err, "expected error for %q", bad)
		}
	})

	t.Run("of time", func(t *testing.T) {
		ts := time.Date(2026, time.March, 31, 23, 59, 0, 0, time.UTC)
		assert.Equal(t, Month("2026-03"), MonthOf(ts))
	})

	t.Run("arithmetic", func(t *testing.T) {
		m := Month("2026-11")
		assert.Equal(t, Month("2026-12"), m.Next())
		assert.Equal(t, Month("2027-01"), m.Add(2))
		assert.Equal(t, Month("2026-10"), m.Prev())
		assert.Equal(t, Month("2024-11"), m.Add(-24))
	})

	t.Run("ordering", func(t *testing.T) {
		assert.True(t, Month("2026-09").Before("2026-10"))
		assert.True(t, Month("2026-12").Before("2027-01"))
		assert.False(t, Month("2026-10").Before("2026-10"))
	})
}
