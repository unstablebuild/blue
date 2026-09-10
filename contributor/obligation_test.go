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

func TestObligationTransitions(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	o := Obligation{
		Account:     "a",
		Month:       "2026-03",
		AmountCents: 1000,
		Currency:    "usd",
		State:       ObligationEarned,
	}

	t.Run("full lifecycle", func(t *testing.T) {
		owed, err := o.Transition(ObligationOwed, now, "quarter closed")
		require.NoError(t, err)
		assert.Equal(t, ObligationOwed, owed.State)

		initiated, err := owed.Transition(ObligationInitiated, now, "tr_1")
		require.NoError(t, err)
		settled, err := initiated.Transition(ObligationSettled, now, "paid")
		require.NoError(t, err)
		assert.Equal(t, ObligationSettled, settled.State)

		require.Len(t, settled.History, 3)
		assert.Equal(t, ObligationEarned, settled.History[0].From)
		assert.Equal(t, ObligationOwed, settled.History[0].To)
		assert.Equal(t, ObligationSettled, settled.History[2].To)
	})

	t.Run("failure returns to owed", func(t *testing.T) {
		owed, err := o.Transition(ObligationOwed, now, "")
		require.NoError(t, err)
		initiated, err := owed.Transition(ObligationInitiated, now, "tr_1")
		require.NoError(t, err)
		back, err := initiated.Transition(ObligationOwed, now,
			"transfer failed")
		require.NoError(t, err)
		assert.Equal(t, ObligationOwed, back.State)
	})

	t.Run("reversal after settlement", func(t *testing.T) {
		settled := o
		settled.State = ObligationSettled
		back, err := settled.Transition(ObligationOwed, now, "reversed")
		require.NoError(t, err)
		assert.Equal(t, ObligationOwed, back.State)
	})

	t.Run("illegal transitions rejected", func(t *testing.T) {
		for _, tc := range []struct{ from, to ObligationState }{
			{ObligationEarned, ObligationInitiated},
			{ObligationEarned, ObligationSettled},
			{ObligationOwed, ObligationSettled},
			{ObligationOwed, ObligationEarned},
			{ObligationSettled, ObligationInitiated},
			{ObligationEstimated, ObligationOwed},
		} {
			bad := o
			bad.State = tc.from
			_, err := bad.Transition(tc.to, now, "")
			assert.ErrorContains(t, err, "illegal transition",
				"%s → %s", tc.from, tc.to)
		}
	})

	t.Run("does not mutate receiver", func(t *testing.T) {
		owed, err := o.Transition(ObligationOwed, now, "")
		require.NoError(t, err)
		_, err = owed.Transition(ObligationInitiated, now, "")
		require.NoError(t, err)
		assert.Equal(t, ObligationEarned, o.State)
		assert.Empty(t, o.History)
		assert.Len(t, owed.History, 1)
	})
}
