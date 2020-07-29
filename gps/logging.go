package gps

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
)

type loggingPositioner struct {
	root     Positioner
	callType string
}

// WithLoggingPositioner wraps a Positioner to provide INFO/ERROR level logging.
func WithLoggingPositioner(pos Positioner, label string) Positioner {
	return loggingPositioner{root: pos, callType: "Position" + label}
}

func (p loggingPositioner) Position(ctx context.Context) (Coordinates, error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, p.callType)

	pos, err := p.root.Position(ctx)
	logging.LogResult(err, attemptAt, traceID, p.callType,
		logging.Field{Key: "Latitude", Value: fmt.Sprintf("%.4f", pos.Latitude)},
		logging.Field{Key: "Longitude", Value: fmt.Sprintf("%.4f", pos.Longitude)},
		logging.Field{Key: "Altitude", Value: fmt.Sprintf("%.2f", pos.Longitude)},
	)
	return pos, err
}

func (p loggingPositioner) Close() error {
	return nil
}

type loggingSender struct {
	root     Sender
	callType string
}

// WithLoggingSender wraps a Sender to provide INFO/ERROR level logging.
func WithLoggingSender(s Sender, label string) Sender {
	return loggingSender{root: s, callType: "Send" + label}
}

func (s loggingSender) Send(ctx context.Context, pos Coordinates) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, s.callType)
	err := s.root.Send(ctx, pos)
	logging.LogResult(err, attemptAt, traceID, s.callType)
	return err
}

func (s loggingSender) Close() error {
	return nil
}

type loggingReceiver struct {
	onOpenCt    string
	onCloseCt   string
	onReceiveCt string
}

// LoggingReceiver returns a Receiver that just logs calls to Receive.
func LoggingReceiver(label string) Receiver {
	return loggingReceiver{
		onReceiveCt: "OnReceive" + label,
	}
}

func (r loggingReceiver) Receive(ctx context.Context, pos Coordinates) error {
	fields := log.Fields{
		logging.KeyCallType: r.onReceiveCt,
		"Latitude":          pos.Latitude,
		"Longitude":         pos.Longitude,
		"Altitude":          pos.Altitude,
	}

	meta, ok := connMetaFromContext(ctx)
	if ok {
		fields["RemoteAddr"] = meta.RemoteAddr.String()
		fields["LocalAddr"] = meta.LocalAddr.String()
	}
	log.WithFields(fields).Info()
	return nil
}
