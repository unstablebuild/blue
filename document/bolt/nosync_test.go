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
