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
	extraFields := [1]logging.Field{{Key: "ID", Value: ID}}
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Create", extraFields[:]...)

	err := s.svc.Create(ctx, ID, data)
	s.logResult(traceID, attemptAt, err, ".Create", extraFields[:]...)

	return err
}

func (s loggingService) logResult(
	traceID trace.ID, attemptAt time.Time,
	err error, method string, extraFields ...logging.Field,
) {
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
	extraFields := [1]logging.Field{{Key: "ID", Value: ID}}
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Set", extraFields[:]...)

	err := s.svc.Set(ctx, ID, data)
	s.logResult(traceID, attemptAt, err, ".Set", extraFields[:]...)
	return err
}

func (s loggingService) Update(
	ctx context.Context, ID string, updates []document.Update,
	preconds ...document.Precondition,
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	extraFields := [1]logging.Field{{Key: "ID", Value: ID}}
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Update", extraFields[:]...)

	err := s.svc.Update(ctx, ID, updates, preconds...)
	s.logResult(traceID, attemptAt, err, ".Update", extraFields[:]...)
	return err
}

func (s loggingService) Get(
	ctx context.Context, ID string, to interface{},
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	extraFields := [1]logging.Field{{Key: "ID", Value: ID}}
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Get", extraFields[:]...)

	err := s.svc.Get(ctx, ID, to)
	s.logResult(traceID, attemptAt, err, ".Get", extraFields[:]...)
	return err
}

func (s loggingService) Delete(ctx context.Context, ID string) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	extraFields := [1]logging.Field{{Key: "ID", Value: ID}}
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Delete", extraFields[:]...)

	err := s.svc.Delete(ctx, ID)
	s.logResult(traceID, attemptAt, err, ".Delete", extraFields[:]...)
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
