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
		defer f.Close()

		store, err := New(f.Name(), "test")
		require.NoError(t, err)

		return store
	})

	doctest.TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
		f, err := os.CreateTemp("", "preconds_bolt_test")
		require.NoError(t, err)
		defer f.Close()

		store, err := New(f.Name(), "test")
		require.NoError(t, err)

		return store
	})

	t.Run("Drop deletes all documents in a collection", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.CreateTemp("", "blue_is_gold")
		require.NoError(t, err)

		defer f.Close()
		store, err := New(f.Name(), "test")
		require.NoError(t, err)

		require.NoError(t, store.Set(ctx, "1", doctest.Alice()))
		require.NoError(t, store.Set(ctx, "2", doctest.Alice()))

		require.NoError(t, store.Drop(ctx))

		it, err := store.List(ctx, nil)
		require.NoError(t, err)
		assert.False(t, it.HasNext())
	})

	t.Run("with two concurrent instances", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.CreateTemp("", "what_is_barnack_test")
		require.NoError(t, err)
		defer f.Close()

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
			for i := 0; i < 20; i++ {
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
