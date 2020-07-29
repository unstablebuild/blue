package gps

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

type key int

var metaKey key

func connMetaFromContext(ctx context.Context) (connectionMetadata, bool) {
	meta, ok := ctx.Value(metaKey).(connectionMetadata)
	return meta, ok
}

func withConnectionMeta(ctx context.Context, meta connectionMetadata) context.Context {
	return context.WithValue(ctx, metaKey, meta)
}

// SendPositionAtCadence schedules p to retrieve and s to send the retrieved
// set of Coordinates at cadence. This function never returns.
func SendPositionAtCadence(p Positioner, s Sender, cadence time.Duration) {
	ticker := time.NewTicker(cadence)
	timeout := time.Duration(float64(cadence.Milliseconds())*0.9) * time.Millisecond

	for {
		<-ticker.C
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		pos, err := p.Position(ctx)
		if err != nil {
			cancel()
			continue
		}

		err = s.Send(ctx, pos)
		if err != nil {
			log.Warnf("failed to send GPS position to server: %v", err)
		}
		cancel()
	}
}
