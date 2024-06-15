// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package logging

import (
	"context"
	"time"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
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
		logging.LogResultTrace(err,
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
