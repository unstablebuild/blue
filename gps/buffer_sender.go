package gps

import (
	"context"
	"time"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
)

const logBufferingCallType = "FallbackToBuffer"

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

func (s *bufferSender) bufferPosition(ctx context.Context, err error, pos Coordinates) error {
	log.WithFields(log.Fields{
		logging.KeyError:    err.Error(),
		logging.KeyCallType: logBufferingCallType,
		logging.KeyStep:     logging.ValueStepAttempt,
	}).Warning()
	return s.buffer.Receive(ctx, pos)
}

func (s *bufferSender) sendBuffered(ctx context.Context) (err error) {
	it, err := s.buffer.List(ctx, time.Time{}, time.Now())
	if err != nil {
		return err
	}

	// avoid logs and also delete operation
	if !it.HasNext() {
		return nil
	}

	var pos Coordinates
	for it.HasNext() {
		err = it.NextTo(&pos)
		if err != nil {
			break
		}
		err = s.root.Send(ctx, pos)
		if err != nil {
			break
		}
	}

	if err != nil {
		log.WithFields(log.Fields{
			logging.KeyError:    err,
			logging.KeyCallType: logBufferingCallType,
			logging.KeyStep:     logging.ValueStepFailure,
		}).Error()
		return err
	}

	log.WithFields(log.Fields{
		logging.KeyCallType: logBufferingCallType,
		logging.KeyStep:     logging.ValueStepSuccess,
	}).Info()

	return s.deleteBuffered(ctx)
}

func (s *bufferSender) Send(ctx context.Context, pos Coordinates) error {
	err := s.root.Send(ctx, pos)
	if err != nil {
		return s.bufferPosition(ctx, err, pos)
	}

	return s.sendBuffered(ctx)
}

func (s *bufferSender) Close() error {
	err1 := s.deleteBuffered(context.Background())
	err2 := s.db.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
