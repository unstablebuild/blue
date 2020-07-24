package gps

import (
	"context"
	"sync"
	"testing"

	"github.com/pion/dtls/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	config = dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			return []byte{0xAB, 0xC1, 0x23}, nil
		},
		PSKIdentityHint:      []byte("GPS DTLS Server Hint"),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}
	ip = "127.0.0.1"
)

func newClientServerPair(t *testing.T, rx ...Receiver) (Sender, *DTLSServer, func()) {
	server, err := NewDTLSServer(ip, 0, config, rx...)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func(t *testing.T) {
		defer wg.Done()
		err := server.Serve()
		require.Error(t, err) // listener closed
	}(t)

	listeningPort := server.Addr().Port

	client, err := NewDTLSSender(ip, listeningPort, config)
	require.NoError(t, err)

	return client, server, func() {
		// wait for ListenAndServe to return
		wg.Wait()
	}
}

func TestSimpleDTLS(t *testing.T) {
	ctx := context.Background()
	pos := Coordinates{Latitude: 1, Longitude: 2, Altitude: 3}

	t.Run("client fails to Send if no server is listening at address", func(t *testing.T) {
		client, err := NewDTLSSender(ip, 0, config)
		require.NoError(t, err)

		err = client.Send(ctx, pos)
		assert.Error(t, err)
	})

	t.Run("client/server establish communication", func(t *testing.T) {
		var meta ConnectionMetadata
		var onClose int
		var onOpen int
		var onReceive int
		mockRx := &testingReceiver{
			onClose: func(_meta ConnectionMetadata) {
				assert.Equal(t, meta, _meta)
				onClose++
			},
			onOpen: func(_meta ConnectionMetadata) {
				meta = _meta
				onOpen++
			},
			onReceive: func(meta ConnectionMetadata, _pos Coordinates) {
				assert.Equal(t, pos, _pos)
				onReceive++
			},
		}

		client, server, cancel := newClientServerPair(t, mockRx)

		err := client.Send(ctx, pos)
		assert.NoError(t, err)

		assert.NoError(t, client.Close())
		assert.NoError(t, server.Close())

		cancel()
		mockRx.lock.Lock()
		defer mockRx.lock.Unlock()

		assert.Equal(t, 1, onClose)
		assert.Equal(t, 1, onOpen)
		assert.Equal(t, 1, onReceive)
	})
}
