package gps

import (
	"context"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
)

// Cache satisfies Receiver by storing the last known position of a DeviceID.
type Cache struct {
	backend document.Service
}

// NewCache allocates storage for a new Cache and initializes it with
// the given persistence backend.
func NewCache(backend document.Service) *Cache {
	c := new(Cache)
	c.backend = backend
	return c
}

// Receive satisfies Receiver.
func (s *Cache) Receive(ctx context.Context, pos Coordinates) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeReceiverLoggingFields("Cache", pos.DeviceID)
	attemptAt := logging.LogAttempt(traceID, "Receive", fields...)

	err := s.backend.Create(ctx, pos.DeviceID, pos)
	logging.LogResultInfo(err, attemptAt, traceID, "Receive", fields...)
	if err != nil {
		return err
	}
	return nil
}

// Get retrieves the last stored Coordinates of the given deviceID.
func (s *Cache) Get(ctx context.Context, deviceID string) (pos Coordinates, err error) {
	err = s.backend.Get(ctx, deviceID, &pos)
	return
}
