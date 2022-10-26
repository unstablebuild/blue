package document

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type nopLocker struct {
}

func (n nopLocker) Lock() {
}
func (n nopLocker) Unlock() {
}

type Segador struct {
	Name   string
	Traits map[string]interface{}
}

func TestSync(t *testing.T) {
	t.Run("goroutine-safe", func(t *testing.T) {
		mem := NewInMemoryService()
		mem.(*inMemoryService).m = nopLocker{}
		svc := Sync(mem)
		defer svc.Close()
		bob := Segador{Name: "hello", Traits: map[string]interface{}{"a": "b"}}

		var wg sync.WaitGroup
		wg.Add(50)
		for i := 0; i < 50; i++ {
			ctx := context.Background()
			myID := uuid.New().String()
			go func() {
				var myBob Segador
				defer wg.Done()
				_ = svc.Create(ctx, myID, bob)
				_ = svc.Get(ctx, myID, &myBob)
				_ = svc.Set(ctx, myID, &myBob)
				_ = svc.Update(ctx, myID, []Update{{FieldPath: []string{"Name"}, Value: "world"}})
				_, _ = svc.List(ctx, nil)
				_ = svc.Delete(ctx, myID)
			}()
		}
		wg.Wait()
	})
}
