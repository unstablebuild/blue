package logging

import (
	"context"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
	"github.com/sirupsen/logrus"
)

type loggingService struct {
	svc         document.Service
	serviceName string
}

// WithLogging wraps a Service to provide instrumentation in the form of logs.
func WithLogging(svc document.Service, name string) document.Service {
	return loggingService{
		svc:         svc,
		serviceName: name,
	}
}

func (s loggingService) Create(
	ctx context.Context, ID string, data interface{},
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Create")

	err := s.svc.Create(ctx, ID, data)
	s.logResult(traceID, attemptAt, err, ".Create")

	return err
}

func (s loggingService) logResult(traceID trace.ID, attemptAt time.Time, err error, method string) {
	switch err {
	case document.ErrNotFound, document.ErrAlreadyExists,
		document.ErrPreconditionFailed, document.ErrPermissionDenied:
		logging.LogResultLevel(logrus.DebugLevel, nil,
			attemptAt, traceID, s.serviceName+method,
			logging.Field{Key: logging.KeyError, Value: err.Error()})
	default:
		logging.LogResult(err, attemptAt, traceID, s.serviceName+method)
	}
}

func (s loggingService) Set(
	ctx context.Context, ID string, data interface{},
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Set")

	err := s.svc.Set(ctx, ID, data)
	s.logResult(traceID, attemptAt, err, ".Set")
	return err
}

func (s loggingService) Update(
	ctx context.Context, ID string, updates []document.Update,
	preconds ...document.Precondition,
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Update")

	err := s.svc.Update(ctx, ID, updates, preconds...)
	s.logResult(traceID, attemptAt, err, ".Update")
	return err
}

func (s loggingService) Get(
	ctx context.Context, ID string, to interface{},
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Get")

	err := s.svc.Get(ctx, ID, to)
	s.logResult(traceID, attemptAt, err, ".Get")
	return err
}

func (s loggingService) Delete(ctx context.Context, ID string) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Delete")

	err := s.svc.Delete(ctx, ID)
	s.logResult(traceID, attemptAt, err, ".Delete")
	return err
}

func (s loggingService) List(ctx context.Context, filters []document.Filter) (
	document.Iterator, error,
) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".List")

	it, err := s.svc.List(ctx, filters)
	s.logResult(traceID, attemptAt, err, ".List")
	return it, err
}

func (s loggingService) Close() error {
	return s.svc.Close()
}
