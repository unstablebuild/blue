package gps

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pion/dtls/v2"
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

func sendBytesConn(ctx context.Context, b []byte, conn *dtls.Conn, ch chan error) error {
	go func() {
		_, err := conn.Write(b)
		ch <- err
	}()

	select {
	case <-ctx.Done():
		conn.SetWriteDeadline(time.Now())
		return <-ch
	case err := <-ch:
		return err
	}
}

func readBytesConn(
	ctx context.Context, readTimeout time.Duration,
	b []byte, conn *dtls.Conn,
	in proto.Message, ch chan error,
) error {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	go func() {
		n, err := conn.Read(b)
		if err != nil {
			ch <- err
		}
		err = proto.Unmarshal(b[:n], in)
		if err != nil {
			ch <- err
		}
		ch <- nil
	}()

	select {
	case <-ctx.Done():
		conn.SetReadDeadline(time.Now())
		return <-ch
	case err := <-ch:
		return err
	}
}
