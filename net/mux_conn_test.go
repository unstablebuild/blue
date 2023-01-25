package net

import (
	"bytes"
	"math"
	"net"
	"os"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
)

func TestMuxDecode(t *testing.T) {
	largePayload := make([]byte, math.MaxInt16)
	for i := 0; i < 1000; i++ {
		smallPayload := []byte("Mors\x00\n")
		copy(largePayload[i*len(smallPayload):], smallPayload)
	}
	tsuite := []struct {
		desc    string
		payload []byte
		id      uint16
	}{
		{"a standard payload id 0", []byte("GOONED"), 0},
		{"a standard payload id non 0", []byte("GOONED"), 2},
		{"a standard payload max id", []byte("GOONED"), math.MaxUint16},
		{"a payload smaller than the header id 0", []byte("\x00"), 0},
		{"a payload smaller than the header max id", []byte("\x00"), math.MaxUint16},
		{"a very large payload id 0", largePayload, 0},
		{"a very large payload max id", largePayload, math.MaxUint16},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			var buf bytes.Buffer
			err := doWriteBuffer(&buf, tcase.payload, tcase.id)
			require.NoError(t, err)
			actualID, actualPayload, err := doReadBuffer(&buf)
			require.NoError(t, err)
			assert.Equal(t, tcase.id, actualID)
			assert.Equal(t, tcase.payload, actualPayload)

			err = doWriteBuffer(&buf, actualPayload, actualID)
			require.NoError(t, err)
			actualID, actualPayload, err = doReadBuffer(&buf)
			require.NoError(t, err)
			assert.Equal(t, tcase.id, actualID)
			assert.Equal(t, tcase.payload, actualPayload)
		})
	}
}

func TestMuxConn(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)
	t.Run("test scaffold", func(t *testing.T) {
		nettest.TestConn(t, mp)
	})
	t.Run("pair of master connections", func(t *testing.T) {
		nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
			x1, x2, mpstop, err := mp()
			if err != nil {
				return nil, nil, nil, err
			}
			c1, err = NewMuxConn(x1, true)
			if err != nil {
				return nil, nil, nil, err
			}
			c2, err = NewMuxConn(x2, false)
			if err != nil {
				return nil, nil, nil, err
			}
			stop = func() {
				_ = c1.Close()
				_ = c2.Close()
				mpstop()
			}
			return c1, c2, stop, nil
		})
	})
	t.Run("pair of muxed connections", func(t *testing.T) {
		nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
			x1, x2, mpstop, err := mp()
			if err != nil {
				return nil, nil, nil, err
			}
			m1, err := NewMuxConn(x1, true)
			if err != nil {
				return nil, nil, nil, err
			}
			m2, err := NewMuxConn(x2, false)
			if err != nil {
				return nil, nil, nil, err
			}
			var id uint16
			id, c1, err = m1.Mux()
			if err != nil {
				return nil, nil, nil, err
			}
			c2, err = m2.Dial(id)
			if err != nil {
				return nil, nil, nil, err
			}
			stop = func() {
				_ = m1.Close()
				_ = m2.Close()
				mpstop()
			}
			return c1, c2, stop, nil
		})
	})
	t.Run("many concurrent muxed connections", func(t *testing.T) {
		x1, x2, mpstop, err := mp()
		require.NoError(t, err)
		m1, err := NewMuxConn(x1, true)
		require.NoError(t, err)
		m2, err := NewMuxConn(x2, false)
		require.NoError(t, err)

		var wg sync.WaitGroup

		n := 10
		wg.Add(n)
		for i := 0; i < n; i++ {
			go nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
				var id uint16
				id, c1, err = m1.Mux()
				if err != nil {
					return nil, nil, nil, err
				}
				c2, err = m2.Dial(id)
				if err != nil {
					return nil, nil, nil, err
				}
				stop = func() {
					defer wg.Done()
					_ = c1.Close()
					_ = c2.Close()
				}
				return c1, c2, stop, nil
			})
		}

		wg.Wait()
		mpstop()
	})
}

func mp() (c1, c2 net.Conn, stop func(), err error) {
	ln, err := nettest.NewLocalListener("unix")
	if err != nil {
		return nil, nil, nil, err
	}

	// Start a connection between two endpoints.
	var err1, err2 error
	done := make(chan bool)
	go func() {
		c2, err2 = ln.Accept()
		close(done)
	}()
	c1, err1 = net.Dial(ln.Addr().Network(), ln.Addr().String())
	<-done

	stop = func() {
		if err1 == nil {
			c1.Close()
		}
		if err2 == nil {
			c2.Close()
		}
		ln.Close()
		os.Remove(ln.Addr().String())
	}

	switch {
	case err1 != nil:
		stop()
		return nil, nil, nil, err1
	case err2 != nil:
		stop()
		return nil, nil, nil, err2
	default:
		return c1, c2, stop, nil
	}
}
