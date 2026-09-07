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
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/doctest"
)

func TestBolt(t *testing.T) {
	doctest.TestDocumentService(t, func(t *testing.T) document.Service {
		f, err := os.CreateTemp("", "barnack_bolt_test")
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		store, err := New(f.Name(), "test")
		require.NoError(t, err)

		return store
	})

	doctest.TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
		f, err := os.CreateTemp("", "preconds_bolt_test")
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		store, err := New(f.Name(), "test")
		require.NoError(t, err)

		return store
	})

	t.Run("Drop deletes all documents in a collection", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.CreateTemp("", "blue_is_gold")
		require.NoError(t, err)

		defer func() { _ = f.Close() }()
		store, err := New(f.Name(), "test")
		require.NoError(t, err)

		require.NoError(t, store.Set(ctx, "1", doctest.Alice()))
		require.NoError(t, store.Set(ctx, "2", doctest.Alice()))

		require.NoError(t, store.Drop(ctx))

		it, err := store.List(ctx, nil)
		require.NoError(t, err)
		assert.False(t, it.HasNext())
	})

	t.Run("ApplyBatch", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.CreateTemp("", "blue_batch_test")
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		store, err := New(f.Name(), "test")
		require.NoError(t, err)

		require.NoError(t, store.Set(ctx, "taken", doctest.Alice()))
		require.NoError(t, store.Set(ctx, "doomed", doctest.Alice()))
		require.NoError(t, store.Set(ctx, "bob", doctest.Bob()))

		results, err := store.ApplyBatch(ctx, []document.BatchOp{
			{Type: document.BatchCreate, ID: "fresh", Doc: doctest.Bob()},
			{Type: document.BatchCreate, ID: "taken", Doc: doctest.Bob()},
			{Type: document.BatchSet, ID: "taken", Doc: doctest.Bob()},
			{Type: document.BatchDelete, ID: "doomed"},
			{Type: document.BatchUpdate, ID: "bob", Updates: []document.Update{
				{FieldPath: []string{"Name"}, Value: "Robert"},
			}},
			{Type: document.BatchUpdate, ID: "bob", Updates: []document.Update{
				{FieldPath: []string{"Name"}, Value: "Bobby"},
			}, Preconditions: []document.Precondition{
				{FieldPath: []string{"Name"}, Value: "Bob"},
			}},
			{Type: document.BatchUpdate, ID: "ghost", Updates: []document.Update{
				{FieldPath: []string{"Name"}, Value: "Casper"},
			}},
		})
		require.NoError(t, err)
		require.Len(t, results, 7)
		assert.NoError(t, results[0].Err)
		assert.ErrorIs(t, results[1].Err, document.ErrAlreadyExists)
		assert.NoError(t, results[2].Err)
		assert.NoError(t, results[3].Err)
		assert.NoError(t, results[4].Err)
		assert.ErrorIs(t, results[5].Err, document.ErrPreconditionFailed)
		assert.ErrorIs(t, results[6].Err, document.ErrNotFound)

		var doc doctest.Segador
		require.NoError(t, store.Get(ctx, "fresh", &doc))
		assert.Equal(t, "Bob", doc.Name)

		require.NoError(t, store.Get(ctx, "bob", &doc))
		assert.Equal(t, "Robert", doc.Name)

		assert.ErrorIs(t, store.Get(ctx, "doomed", &doc), document.ErrNotFound)
	})

	t.Run("with two concurrent instances", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.CreateTemp("", "what_is_barnack_test")
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		store1, err := New(f.Name(), "test-1")
		require.NoError(t, err)

		store2, err := New(f.Name(), "test-2")
		require.NoError(t, err)

		one := "daas"
		two := "postmates"
		e1 := doctest.Alice()
		e2 := doctest.Bob()

		t.Run("is safe to use two instances of the service with same database file", func(t *testing.T) {
			var wg sync.WaitGroup
			var m sync.Mutex
			for range 20 {
				wg.Add(2)
				go func() {
					defer wg.Done()
					err := store1.Set(ctx, one, &e1)
					m.Lock()
					defer m.Unlock()
					assert.NoError(t, err)
				}()
				go func() {
					defer wg.Done()
					err := store2.Set(ctx, two, &e2)
					m.Lock()
					defer m.Unlock()
					assert.NoError(t, err)
				}()
			}

			wg.Wait()

			var r1 doctest.Segador
			var r2 doctest.Segador

			wg.Add(2)
			go func() {
				defer wg.Done()
				err := store1.Get(ctx, one, &r1)
				m.Lock()
				defer m.Unlock()
				assert.NoError(t, err)
			}()
			go func() {
				defer wg.Done()
				err := store2.Get(ctx, two, &r2)
				m.Lock()
				defer m.Unlock()
				assert.NoError(t, err)
			}()

			wg.Wait()

			assert.EqualValues(t, e1, r1)
			assert.EqualValues(t, e2, r2)
		})

		t.Run("List returns data of its corresponding instance", func(t *testing.T) {
			for _, store := range []document.Service{store1, store2} {
				it, err := store.List(ctx, nil)
				require.NoError(t, err)

				var r1 doctest.Segador
				require.True(t, it.HasNext())
				assert.NoError(t, it.NextTo(&r1))
				assert.False(t, it.HasNext())
			}
		})
	})

}

// TestCloseSharedDBLifecycle verifies that closing one Store does not pull
// the process-shared *bolt.DB out from under sibling Stores opened against
// the same path. Without reference counting, the survivor fails with
// "database not open".
func TestCloseSharedDBLifecycle(t *testing.T) {
	ctx := context.Background()
	f, err := os.CreateTemp("", "bolt_close_lifecycle_test")
	require.NoError(t, err)
	defer func() { _ = os.Remove(f.Name()) }()
	require.NoError(t, f.Close())

	store1, err := New(f.Name(), "test")
	require.NoError(t, err)
	store2, err := New(f.Name(), "test")
	require.NoError(t, err)

	require.NoError(t, store2.Set(ctx, "1", doctest.Alice()))
	require.NoError(t, store1.Close())

	var got doctest.Segador
	require.NoError(t, store2.Get(ctx, "1", &got),
		"closing one store must not close the DB shared with siblings")

	require.NoError(t, store2.Close())

	store3, err := New(f.Name(), "test")
	require.NoError(t, err)
	require.NoError(t, store3.Get(ctx, "1", &got),
		"a fresh Store must reopen the DB after the last Close")
	require.NoError(t, store3.Close())
}
