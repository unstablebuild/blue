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

package semanticrpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinContexts_cancelParentA(t *testing.T) {
	t.Parallel()
	a, cancelA := context.WithCancel(context.Background())
	b, cancelB := context.WithCancel(context.Background())
	defer cancelB()

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	cancelA()

	select {
	case <-joined.Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("joined context was not done after parent a canceled")
	}
}

func TestJoinContexts_cancelParentB(t *testing.T) {
	t.Parallel()
	a, cancelA := context.WithCancel(context.Background())
	b, cancelB := context.WithCancel(context.Background())
	defer cancelA()

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	cancelB()

	select {
	case <-joined.Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("joined context was not done after parent b canceled")
	}
}

func TestJoinContexts_cancelDirectly(t *testing.T) {
	t.Parallel()
	a, cancelA := context.WithCancel(context.Background())
	b, cancelB := context.WithCancel(context.Background())
	defer cancelA()
	defer cancelB()

	joined, cancelJ := joinContexts(a, b)

	cancelJ()

	select {
	case <-joined.Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("joined context was not done after direct cancel")
	}

	assert.ErrorIs(t, joined.Err(), context.Canceled)
}

func TestJoinContexts_deadlineReturnsEarliest(t *testing.T) {
	t.Parallel()
	early := time.Now().Add(1 * time.Hour)
	late := time.Now().Add(2 * time.Hour)

	a, cancelA := context.WithDeadline(context.Background(), early)
	b, cancelB := context.WithDeadline(context.Background(), late)
	defer cancelA()
	defer cancelB()

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	dl, ok := joined.Deadline()
	require.True(t, ok, "expected a deadline")
	assert.True(t, dl.Equal(early),
		"deadline should be the earlier of the two parents")
}

func TestJoinContexts_deadlineReturnsEarliestReversed(t *testing.T) {
	t.Parallel()
	early := time.Now().Add(1 * time.Hour)
	late := time.Now().Add(2 * time.Hour)

	a, cancelA := context.WithDeadline(context.Background(), late)
	b, cancelB := context.WithDeadline(context.Background(), early)
	defer cancelA()
	defer cancelB()

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	dl, ok := joined.Deadline()
	require.True(t, ok, "expected a deadline")
	assert.True(t, dl.Equal(early),
		"deadline should be the earlier of the two parents")
}

func TestJoinContexts_deadlineFromOnlyParent(t *testing.T) {
	t.Parallel()
	deadline := time.Now().Add(1 * time.Hour)

	a, cancelA := context.WithDeadline(context.Background(), deadline)
	b := context.Background() // no deadline
	defer cancelA()

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	dl, ok := joined.Deadline()
	require.True(t, ok, "expected a deadline from parent a")
	assert.True(t, dl.Equal(deadline),
		"deadline should match the only parent that has one")
}

func TestJoinContexts_deadlineFromOnlyParentB(t *testing.T) {
	t.Parallel()
	deadline := time.Now().Add(1 * time.Hour)

	a := context.Background() // no deadline
	b, cancelB := context.WithDeadline(context.Background(), deadline)
	defer cancelB()

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	dl, ok := joined.Deadline()
	require.True(t, ok, "expected a deadline from parent b")
	assert.True(t, dl.Equal(deadline),
		"deadline should match the only parent that has one")
}

func TestJoinContexts_noDeadline(t *testing.T) {
	t.Parallel()
	a := context.Background()
	b := context.Background()

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	_, ok := joined.Deadline()
	assert.False(t, ok, "expected no deadline")
}

type ctxKey string

func TestJoinContexts_valueFromFirstContext(t *testing.T) {
	t.Parallel()
	key := ctxKey("shared")
	a := context.WithValue(context.Background(), key, "from-a")
	b := context.WithValue(context.Background(), key, "from-b")

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	assert.Equal(t, "from-a", joined.Value(key),
		"value should come from context a when both have the key")
}

func TestJoinContexts_valueFallsBackToSecondContext(t *testing.T) {
	t.Parallel()
	key := ctxKey("only-in-b")
	a := context.Background()
	b := context.WithValue(context.Background(), key, "from-b")

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	assert.Equal(t, "from-b", joined.Value(key),
		"value should fall back to context b when a has no value")
}

func TestJoinContexts_valueNilWhenAbsent(t *testing.T) {
	t.Parallel()
	a := context.Background()
	b := context.Background()

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	assert.Nil(t, joined.Value(ctxKey("missing")),
		"value should be nil when neither context has the key")
}

func TestJoinContexts_errBeforeAndAfterDone(t *testing.T) {
	t.Parallel()
	a, cancelA := context.WithCancel(context.Background())
	b, cancelB := context.WithCancel(context.Background())
	defer cancelB()

	joined, cancelJ := joinContexts(a, b)
	defer cancelJ()

	assert.NoError(t, joined.Err(),
		"err should be nil before the context is done")

	cancelA()

	select {
	case <-joined.Done():
	case <-time.After(time.Second):
		t.Fatal("joined context was not done after parent a canceled")
	}

	assert.ErrorIs(t, joined.Err(), context.Canceled,
		"err should reflect the parent that caused cancellation")
}
