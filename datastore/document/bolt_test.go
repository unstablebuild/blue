package document

import (
	"context"
	"io/ioutil"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBolt(t *testing.T) {
	testDatastore(t, func(t *testing.T) Service {
		f, err := ioutil.TempFile("", "barnack_bolt_test")
		require.NoError(t, err)
		defer f.Close()

		store, err := NewBolt(f.Name(), "test")
		require.NoError(t, err)

		return store
	})

	t.Run("is safe to use two instances of the service with same database file", func(t *testing.T) {
		ctx := context.Background()
		f, err := ioutil.TempFile("", "what_is_barnack_test")
		require.NoError(t, err)
		defer f.Close()

		store1, err := NewBolt(f.Name(), "test-1")
		require.NoError(t, err)

		store2, err := NewBolt(f.Name(), "test-2")
		require.NoError(t, err)

		one := "daas"
		two := "postmates"
		e1 := alice
		e2 := bob

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

		var r1 segador
		var r2 segador

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
}
