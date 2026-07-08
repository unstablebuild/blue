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

package bolt

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noSyncDoc struct {
	Value string
}

func TestNoSyncRequested(t *testing.T) {
	ctx := context.Background()
	assert.False(t, NoSyncRequested(ctx))
	assert.True(t, NoSyncRequested(ContextWithNoSync(ctx)))
	assert.False(t, NoSyncRequested(context.Background()))
}

func TestContextWithNoSyncRelaxesCommitDurability(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nosync.db")
	store, err := New(path, "test")
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	relaxed := ContextWithNoSync(ctx)

	require.NoError(t, store.Set(ctx, "a", noSyncDoc{Value: "1"}))
	assert.False(t, store.db.NoSync, "plain contexts must commit synced")

	require.NoError(t, store.Set(relaxed, "b", noSyncDoc{Value: "2"}))
	assert.True(t, store.db.NoSync, "relaxed contexts must skip fsync")

	require.NoError(t, store.Delete(relaxed, "a"))
	assert.True(t, store.db.NoSync)

	// The durability barrier: the next plain write restores synced
	// commits, flushing everything before it.
	require.NoError(t, store.Set(ctx, "c", noSyncDoc{Value: "3"}))
	assert.False(t, store.db.NoSync)

	var got noSyncDoc
	require.NoError(t, store.Get(ctx, "b", &got))
	assert.Equal(t, "2", got.Value)
	require.NoError(t, store.Get(ctx, "c", &got))
	assert.Equal(t, "3", got.Value)
}

// Mixed-durability writers on stores sharing one database path must
// not race on the commit flag: flips only happen while no commit is in
// flight.
func TestNoSyncConcurrentMixedWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nosync.db")
	a, err := New(path, "colla")
	require.NoError(t, err)
	defer func() { _ = a.Close() }()
	b, err := New(path, "collb")
	require.NoError(t, err)
	defer func() { _ = b.Close() }()

	ctx := context.Background()
	relaxed := ContextWithNoSync(ctx)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store, wctx := a, ctx
			if i%2 == 0 {
				store, wctx = b, relaxed
			}
			for j := range 25 {
				id := fmt.Sprintf("doc-%d-%d", i, j)
				assert.NoError(t, store.Set(wctx, id, noSyncDoc{Value: id}))
			}
		}()
	}
	wg.Wait()

	for i := range 4 {
		store := a
		if i%2 == 0 {
			store = b
		}
		for j := range 25 {
			id := fmt.Sprintf("doc-%d-%d", i, j)
			var got noSyncDoc
			require.NoError(t, store.Get(ctx, id, &got))
			assert.Equal(t, id, got.Value)
		}
	}
}

func BenchmarkSetSynced(b *testing.B) {
	benchmarkSet(b, context.Background())
}

func BenchmarkSetNoSync(b *testing.B) {
	benchmarkSet(b, ContextWithNoSync(context.Background()))
}

func benchmarkSet(b *testing.B, ctx context.Context) {
	path := filepath.Join(b.TempDir(), "nosync-bench.db")
	store, err := New(path, "bench")
	require.NoError(b, err)
	defer func() { _ = store.Close() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.Set(ctx, fmt.Sprintf("doc-%d", i),
			noSyncDoc{Value: "v"}); err != nil {
			b.Fatal(err)
		}
	}
}
