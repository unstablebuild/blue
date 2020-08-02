package gps

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pion/dtls/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	dtlsConfig = dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			return []byte{0xAB, 0xC1, 0x23}, nil
		},
		PSKIdentityHint:      []byte("GPS DTLS Server Hint"),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}
	ip = "127.0.0.1"
)

func newClientServerPair(t *testing.T, port int, rx ...Receiver) (
	Sender, *DTLSServer, int, func(),
) {
	server, err := NewDTLSServer(ip, port, dtlsConfig, rx...)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func(t *testing.T) {
		defer wg.Done()
		err := server.Serve()
		require.Error(t, err) // listener closed
	}(t)

	listeningPort := server.Addr().Port

	client, err := NewDTLSSender(ip, listeningPort, dtlsConfig)
	require.NoError(t, err)

	return client, server, listeningPort, func() {
		// wait for ListenAndServe to return
		wg.Wait()
	}
}

func TestSimpleDTLS(t *testing.T) {
	ctx := context.Background()
	pos := Coordinates{
		DeviceID:  "Stinson",
		Latitude:  1,
		Longitude: 2,
		Altitude:  3,
		UnixTime:  time.Now().Unix(),
	}

	t.Run("client fails to Send if no server is listening at address", func(t *testing.T) {
		client, err := NewDTLSSender(ip, 0, dtlsConfig)
		require.NoError(t, err)

		err = client.Send(ctx, pos)
		assert.Error(t, err)
	})

	t.Run("client/server establish communication", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)
		mockRx := &testingReceiver{
			onReceive: func(ctx context.Context, _pos Coordinates) error {
				_, ok := connMetaFromContext(ctx)
				assert.True(t, ok)
				assert.Equal(t, pos, _pos)
				wg.Done()
				return nil
			},
		}

		client, server, _, cancel := newClientServerPair(t, 0, mockRx)

		err := client.Send(ctx, pos)
		assert.NoError(t, err)

		wg.Wait()

		assert.NoError(t, client.Close())
		assert.NoError(t, server.Close())

		cancel()
	})

	t.Run("client respects context deadline", func(t *testing.T) {
		ctx, cancelTl := context.WithDeadline(context.Background(), time.Now())
		client, server, _, cancel := newClientServerPair(t, 0)
		defer cancelTl()
		defer cancel()
		defer server.Close()
		defer client.Close()

		err := client.Send(ctx, pos)
		assert.Error(t, err)
	})
}
