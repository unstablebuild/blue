package gps

import (
	"context"
	"time"

	"github.com/ernestrc/blue/datastore/document"
)

// 7 days of minute-level buffering
var bufferOpts = []Option{
	WithFixedSize(7 * 24 * 60),
	WithDownSampling(time.Minute),
}

type bufferSender struct {
	root Sender
	db   document.DroppableService

	buffer *Store
}

// WithBufferFallback wraps a sender to provide buffering of data points
// upon errors. When sending successfully is resumed buffered positions are
// transmitted in bulk.
func WithBufferFallback(s Sender, db document.DroppableService) Sender {
	return &bufferSender{
		root:   s,
		db:     db,
		buffer: NewStore(db, bufferOpts...),
	}
}

func (s *bufferSender) deleteBuffered(ctx context.Context) error {
	return s.db.Drop(ctx)
}

func (s *bufferSender) bufferPosition(ctx context.Context, pos Coordinates) error {
	return s.buffer.Receive(ctx, pos)
}

func (s *bufferSender) sendBuffered(ctx context.Context) error {
	it, err := s.buffer.List(ctx, time.Time{}, time.Now())
	if err != nil {
		return err
	}

	var pos Coordinates
	for it.HasNext() {
		err := it.NextTo(&pos)
		if err != nil {
			return err
		}
		err = s.root.Send(ctx, pos)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *bufferSender) Send(ctx context.Context, pos Coordinates) error {
	err := s.root.Send(ctx, pos)
	if err != nil {
		return s.bufferPosition(ctx, pos)
	}

	err = s.sendBuffered(ctx)
	if err == nil {
		return s.deleteBuffered(ctx)
	}

	return nil
}

func (s *bufferSender) Close() error {
	err1 := s.deleteBuffered(context.Background())
	err2 := s.db.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
