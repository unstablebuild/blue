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
)

// Sync returns a mutual exclusion document.Service.
// All calls are serialized preventing concurrent access to the given
// underlying Service.
func Sync(svc Service) Service {
	return SyncWithLocker(svc, new(sync.Mutex))
}

func SyncWithLocker(svc Service, locker sync.Locker) Service {
	return &syncService{svc: svc, mu: locker}
}

// RWSync returns a mutual read/write exclusion document.Service.
// Create, Update, and Delete calls are serialized and will
// prevent Get and List to access the underlying documents while running
// but if there are no writes reads can run concurrently.
func RWSync(svc Service) Service {
	return &rwSyncService{svc: svc}
}

type rwSyncService struct {
	mu  sync.RWMutex
	svc Service
}

func (s *rwSyncService) Create(ctx context.Context, ID string, doc interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Create(ctx, ID, doc)
}

func (s *rwSyncService) Set(ctx context.Context, ID string, doc interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Set(ctx, ID, doc)
}

func (s *rwSyncService) Update(
	ctx context.Context, ID string, updates []Update,
	preconds ...Precondition,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Update(ctx, ID, updates, preconds...)
}

func (s *rwSyncService) Get(ctx context.Context, ID string, doc interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.svc.Get(ctx, ID, doc)
}

func (s *rwSyncService) Delete(ctx context.Context, ID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Delete(ctx, ID)
}

func (s *rwSyncService) List(ctx context.Context, filters []Filter) (Iterator, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.svc.List(ctx, filters)
}

func (s *rwSyncService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Close()
}

type syncService struct {
	mu  sync.Locker
	svc Service
}

func (s *syncService) Create(ctx context.Context, ID string, doc interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Create(ctx, ID, doc)
}

func (s *syncService) Set(ctx context.Context, ID string, doc interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Set(ctx, ID, doc)
}

func (s *syncService) Update(
	ctx context.Context, ID string, updates []Update,
	preconds ...Precondition,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Update(ctx, ID, updates, preconds...)
}

func (s *syncService) Get(ctx context.Context, ID string, doc interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Get(ctx, ID, doc)
}

func (s *syncService) Delete(ctx context.Context, ID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Delete(ctx, ID)
}

func (s *syncService) List(ctx context.Context, filters []Filter) (Iterator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.List(ctx, filters)
}

func (s *syncService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Close()
}
