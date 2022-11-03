package logging

import (
	"context"
	"reflect"
	"strings"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
)

type loggingService struct {
	svc         document.Service
	serviceName string
}

func getServiceName(tpe reflect.Type) string {
	return strings.Replace(tpe.String(), "*document.", "", 1)
}

// WithLogging wraps a Service to provide instrumentation in the form of logs.
func WithLogging(svc document.Service) document.Service {
	return loggingService{
		svc:         svc,
		serviceName: getServiceName(reflect.TypeOf(svc)),
	}
}

func (s loggingService) Create(
	ctx context.Context, ID string, data interface{},
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Create")

	err := s.svc.Create(ctx, ID, data)
	logging.LogResult(err, attemptAt, traceID, s.serviceName+".Create")

	return err
}

func (s loggingService) Set(
	ctx context.Context, ID string, data interface{},
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Set")

	err := s.svc.Set(ctx, ID, data)
	logging.LogResult(err, attemptAt, traceID, s.serviceName+".Set")

	return err
}

func (s loggingService) Update(
	ctx context.Context, ID string, updates []document.Update,
	preconds ...document.Precondition,
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Update")

	err := s.svc.Update(ctx, ID, updates, preconds...)
	logging.LogResult(err, attemptAt, traceID, s.serviceName+".Update")

	return err
}

func (s loggingService) Get(
	ctx context.Context, ID string, to interface{},
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Get")

	err := s.svc.Get(ctx, ID, to)
	logging.LogResult(err, attemptAt, traceID, s.serviceName+".Get")

	return err
}

func (s loggingService) Delete(ctx context.Context, ID string) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".Delete")

	err := s.svc.Delete(ctx, ID)
	logging.LogResult(err, attemptAt, traceID, s.serviceName+".Delete")

	return err
}

func (s loggingService) List(ctx context.Context, filters []document.Filter) (
	document.Iterator, error,
) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.serviceName+".List")

	it, err := s.svc.List(ctx, filters)
	logging.LogResult(err, attemptAt, traceID, s.serviceName+".List")

	return it, err
}

func (s loggingService) Close() error {
	return s.svc.Close()
}
