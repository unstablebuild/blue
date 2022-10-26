package document

import (
	"context"
	"sync"
)

type syncService struct {
	mu  sync.RWMutex
	svc Service
}

// Sync returns a mutual read/write exclusion document.Service.
// Create, Update, and Delete calls are serialized and will
// prevent Get and List to access the underlying documents while running
// but if there are no writes reads can run concurrently.
func Sync(svc Service) Service {
	return &syncService{svc: svc}
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

func (s *syncService) Update(ctx context.Context, ID string, updates []Update) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Update(ctx, ID, updates)
}

func (s *syncService) Get(ctx context.Context, ID string, doc interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.svc.Get(ctx, ID, doc)
}

func (s *syncService) Delete(ctx context.Context, ID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Delete(ctx, ID)
}

func (s *syncService) List(ctx context.Context, filters []Filter) (Iterator, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.svc.List(ctx, filters)
}

func (s *syncService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Close()
}
