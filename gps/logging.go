package gps

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
)

const (
	positionCallType = "gps.PositionerPosition"
	sendCallType     = "gps.SenderSend"
	receiveCallType  = "Receive"
)

type loggingPositioner struct {
	root  Positioner
	label string
}

// WithLoggingPositioner wraps a Positioner to provide INFO/ERROR level logging.
func WithLoggingPositioner(pos Positioner, label string) Positioner {
	return loggingPositioner{root: pos, label: label}
}

func (p loggingPositioner) Position(ctx context.Context) (Coordinates, error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, positionCallType,
		logging.Field{Key: "Label", Value: p.label})

	pos, err := p.root.Position(ctx)
	logging.LogResultInfo(err, attemptAt, traceID, positionCallType,
		logging.Field{Key: logging.KeyClass, Value: p.label},
		logging.Field{Key: "Latitude", Value: fmt.Sprintf("%.4f", pos.Latitude)},
		logging.Field{Key: "Longitude", Value: fmt.Sprintf("%.4f", pos.Longitude)},
		logging.Field{Key: "Altitude", Value: fmt.Sprintf("%.2f", pos.Altitude)},
		logging.Field{Key: "DeviceID", Value: pos.DeviceID},
	)
	return pos, err
}

func (p loggingPositioner) Close() error {
	return nil
}

type loggingSender struct {
	root  Sender
	label string
}

// WithLoggingSender wraps a Sender to provide INFO/ERROR level logging.
func WithLoggingSender(s Sender, label string) Sender {
	return loggingSender{root: s, label: label}
}

func (s loggingSender) Send(ctx context.Context, pos Coordinates) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := []logging.Field{
		logging.Field{Key: "DeviceID", Value: pos.DeviceID},
		logging.Field{Key: logging.KeyClass, Value: s.label},
	}
	attemptAt := logging.LogAttempt(traceID, sendCallType, fields...)
	err := s.root.Send(ctx, pos)
	logging.LogResult(err, attemptAt, traceID, sendCallType, fields...)
	return err
}

func (s loggingSender) Close() error {
	return nil
}

type loggingReceiver struct {
	label string
}

// LoggingReceiver returns a Receiver that just logs calls to Receive.
func LoggingReceiver(label string) Receiver {
	return loggingReceiver{label: label}
}

func (r loggingReceiver) Receive(ctx context.Context, pos Coordinates) error {
	fields := log.Fields{
		logging.KeyCallType: receiveCallType,
		"Latitude":          pos.Latitude,
		"Longitude":         pos.Longitude,
		"Altitude":          pos.Altitude,
		logging.KeyClass:    r.label,
	}

	meta, ok := connMetaFromContext(ctx)
	if ok {
		fields["RemoteAddr"] = meta.RemoteAddr.String()
		fields["LocalAddr"] = meta.LocalAddr.String()
	}
	log.WithFields(fields).Info()
	return nil
}

func makeReceiverLoggingFields(class string, deviceID string) []logging.Field {
	return []logging.Field{
		logging.Field{Key: logging.KeyClass, Value: class},
		logging.Field{Key: "DeviceID", Value: deviceID},
	}
}
