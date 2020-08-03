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
	config  config
}

// NewCache allocates storage for a new Cache and initializes it with
// the given persistence backend and list of Option. Note that
// cache only takes WithLoggingLabel Option; the rest are silently ignored.
func NewCache(backend document.Service, opts ...Option) *Cache {
	c := new(Cache)
	c.backend = backend

	for _, o := range opts {
		o(&c.config)
	}
	return c
}

func (s *Cache) getLoggingLabel() string {
	if s.config.label == "" {
		return "Cache"
	}
	return s.config.label
}

// Receive satisfies Receiver.
func (s *Cache) Receive(ctx context.Context, pos Coordinates) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeReceiverLoggingFields(s.getLoggingLabel(), pos.DeviceID)
	attemptAt := logging.LogAttempt(traceID, receiveCallType, fields...)

	err := s.backend.Set(ctx, pos.DeviceID, pos)
	logging.LogResultInfo(err, attemptAt, traceID, receiveCallType, fields...)
	if err != nil {
		return err
	}
	return nil
}

// Get retrieves the last stored Coordinates of the given deviceID.
func (s *Cache) Get(ctx context.Context, deviceID string) (pos Coordinates, err error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := []logging.Field{
		logging.Field{Key: logging.KeyClass, Value: s.getLoggingLabel()},
		logging.Field{Key: "DeviceID", Value: deviceID},
	}
	attemptAt := logging.LogAttempt(traceID, "Get", fields...)

	err = s.backend.Get(ctx, deviceID, &pos)
	logging.LogResult(err, attemptAt, traceID, "Get", fields...)
	return
}

// GetAll retrieves the last stored Coordinates of all known devices.
func (s *Cache) GetAll(ctx context.Context) ([]Coordinates, error) {
	it, err := s.backend.List(ctx, nil)
	if err != nil {
		return nil, err
	}

	var pos Coordinates
	ret := make([]Coordinates, 0)
	for it.HasNext() {
		err := it.NextTo(&pos)
		if err != nil {
			return nil, err
		}
		ret = append(ret, pos)
	}
	return ret, nil
}
