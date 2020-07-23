package gps

import (
	"context"
	"fmt"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/logging/trace"
)

type loggingPositioner struct {
	root  Positioner
	label string
}

// WithLogging wraps a Positioner to provide INFO/ERROR level logging.
func WithLogging(pos Positioner, label string) Positioner {
	return loggingPositioner{root: pos, label: label}
}

func (p loggingPositioner) Position(ctx context.Context) (Coordinates, error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	attemptAt := logging.LogAttempt(traceID, p.label+"Position")

	pos, err := p.root.Position(ctx)
	logging.LogResult(err, attemptAt, traceID,
		p.label+"Position",
		logging.Field{Key: "Latitude", Value: fmt.Sprintf("%.4f", pos.Latitude)},
		logging.Field{Key: "Longitude", Value: fmt.Sprintf("%.4f", pos.Longitude)},
		logging.Field{Key: "Altitude", Value: fmt.Sprintf("%.2f", pos.Longitude)},
	)
	return pos, err
}

func (p loggingPositioner) Close() error {
	return nil
}
