package gps

import (
	"context"
	"fmt"
	"time"

	"github.com/ernestrc/blue/datastore"
	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
)

type config struct {
	resolution time.Duration
	fixedSize  int64
}

// Option enables a Store to implement
// different time-based persistence and sampling policies.
type Option func(*config)

// WithFixedSize returns an option that sets a maximum number of
// gps Coordinates stored for a given DeviceID.
func WithFixedSize(records int64) Option {
	return func(cfg *config) {
		cfg.fixedSize = records
	}
}

// WithDownSampling returns an option that down samples gps Coordinates
// to the given time resolution.
func WithDownSampling(resolution time.Duration) Option {
	return func(cfg *config) {
		cfg.resolution = resolution
	}
}

// Store store satisfies Receiver interface to persist a time series
// of a device's GPS position.
type Store struct {
	// not the best storage for GPS data, but since we don't
	// need to run geo queries for now, we should be fine.
	backend document.Service
	config  config
}

// NewStore allocates storage for a new Store and
// initializes it with the given document.Service backend and options.
func NewStore(backend document.Service, opts ...Option) *Store {
	r := new(Store)
	for _, opt := range opts {
		opt(&r.config)
	}
	r.backend = backend
	return r
}

func (s *Store) makeUniqueID(pos Coordinates) string {
	ts := time.Unix(pos.UnixTime, 0).Truncate(s.config.resolution).Unix()
	if s.config.fixedSize != 0 {
		ts %= s.config.fixedSize
	}
	return fmt.Sprintf("%s:%d", pos.DeviceID, ts)
}

// Receive satisfies Receiver.
func (s *Store) Receive(ctx context.Context, pos Coordinates) error {
	id := s.makeUniqueID(pos)

	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeReceiverLoggingFields("Store", pos.DeviceID)
	fields = append(fields, logging.Field{Key: "GeneratedID", Value: id})
	attemptAt := logging.LogAttempt(traceID, "Receive", fields...)

	err := s.backend.Set(ctx, id, pos)
	logging.LogResult(err, attemptAt, traceID, "Receive", fields...)
	return err
}

func withFilterTime(
	in []document.Filter, from, to time.Time,
) []document.Filter {
	in = datastore.WithFilter(in,
		[]string{"UnixTime"}, from.Unix(), document.OpGreaterThanEqual)
	return datastore.WithFilter(in,
		[]string{"UnixTime"}, to.Unix(), document.OpLessThan)
}

func withFilterDeviceID(
	in []document.Filter, deviceID string,
) []document.Filter {
	return datastore.WithFilter(in, []string{"DeviceID"}, deviceID, document.OpEqual)
}

// ListDevice lists all the known positions of a device between from and to.
func (s *Store) ListDevice(
	ctx context.Context, deviceID string, from, to time.Time,
) (document.Iterator, error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeReceiverLoggingFields("Store", deviceID)
	fields = append(fields, []logging.Field{
		logging.Field{Key: "From", Value: from.String()},
		logging.Field{Key: "To", Value: to.String()}}...)
	attemptAt := logging.LogAttempt(traceID, "ListDevice", fields...)

	filters := withFilterTime(nil, from, to)
	filters = withFilterDeviceID(filters, deviceID)

	iter, err := s.backend.List(ctx, filters)
	logging.LogResult(err, attemptAt, traceID, "ListDevice", fields...)
	return iter, err
}

// List lists all the known positions of all devices between from and to.
func (s *Store) List(ctx context.Context, from, to time.Time) (
	document.Iterator, error,
) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := []logging.Field{
		logging.Field{Key: logging.KeyClass, Value: "Store"},
		logging.Field{Key: "From", Value: from.String()},
		logging.Field{Key: "To", Value: to.String()},
	}
	attemptAt := logging.LogAttempt(traceID, "List", fields...)

	iter, err := s.backend.List(ctx, withFilterTime(nil, from, to))
	logging.LogResult(err, attemptAt, traceID, "List", fields...)
	return iter, err
}
