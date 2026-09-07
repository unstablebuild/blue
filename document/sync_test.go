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
		defer func() { _ = svc.Close() }()
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
