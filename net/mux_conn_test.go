package net

import (
	"bytes"
	"io"
	"math"
	"math/rand"
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
			err := doWriteWriter(&buf, tcase.payload, tcase.id)
			require.NoError(t, err)
			actualID, idOk, actualPayload, err := doReadReader(&buf)
			require.NoError(t, err)
			assert.True(t, idOk)
			assert.Equal(t, tcase.id, actualID)
			assert.Equal(t, tcase.payload, actualPayload)

			err = doWriteWriter(&buf, actualPayload, actualID)
			require.NoError(t, err)
			actualID, idOk, actualPayload, err = doReadReader(&buf)
			require.NoError(t, err)
			assert.True(t, idOk)
			assert.Equal(t, tcase.id, actualID)
			assert.Equal(t, tcase.payload, actualPayload)
		})
	}
}

func init() {
	logrus.SetLevel(logrus.TraceLevel)
}

func TestMuxConn(t *testing.T) {
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
			c2, err = m2.DialConn(id)
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
	t.Run("pair of muxed connections via accept", func(t *testing.T) {
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
			var cc1 net.Conn
			var cerr error
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				cc1, cerr = m1.Accept()
			}()
			c2, err = m2.Dial()
			if err != nil {
				return nil, nil, nil, err
			}
			wg.Wait()
			if cerr != nil {
				return nil, nil, nil, cerr
			}
			c1 = cc1
			stop = func() {
				_ = m1.Close()
				_ = m2.Close()
				mpstop()
			}
			return c1, c2, stop, nil
		})
	})

	t.Run("pair of muxed connections over a muxed connection", func(t *testing.T) {
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
			c2, err = m2.DialConn(id)
			if err != nil {
				return nil, nil, nil, err
			}
			m3, err := NewMuxConn(c1, true)
			if err != nil {
				return nil, nil, nil, err
			}
			m4, err := NewMuxConn(c2, false)
			if err != nil {
				return nil, nil, nil, err
			}

			stop = func() {
				_ = c1.Close()
				_ = c2.Close()
				_ = m1.Close()
				_ = m2.Close()
				mpstop()
			}
			return m3, m4, stop, nil
		})
	})
}

const (
	smallBuffer = 64
	largeBuffer = 64 * 1024
)

func BenchmarkMuxConnReadSideSmallBuffer(b *testing.B) {
	benchmarkMuxConnReadSide(b, smallBuffer)
}

func BenchmarkMuxConnReadSideLargeBuffer(b *testing.B) {
	benchmarkMuxConnReadSide(b, largeBuffer)
}

func BenchmarkBaselineConnReadSideSmallBuffer(b *testing.B) {
	c1, c2, mpstop, err := mp()
	if err != nil {
		b.Errorf("scaffold")
	}
	benchmarkConnReadSide(b, c1, c2, smallBuffer)
	mpstop()
}

func BenchmarkBaselineConnReadSideLargeBuffer(b *testing.B) {
	c1, c2, mpstop, err := mp()
	if err != nil {
		b.Errorf("scaffold")
	}
	benchmarkConnReadSide(b, c1, c2, largeBuffer)
	mpstop()
}

func BenchmarkMuxConnWriteSideSmallBuffer(b *testing.B) {
	benchmarkMuxConnWriteSide(b, smallBuffer)
}

func BenchmarkMuxConnWriteSideLargeBuffer(b *testing.B) {
	benchmarkMuxConnWriteSide(b, largeBuffer)
}

func BenchmarkBaselineConnWriteSideSmallBuffer(b *testing.B) {
	c1, c2, mpstop, err := mp()
	if err != nil {
		b.Errorf("scaffold")
	}
	benchmarkConnWriteSide(b, c1, c2, smallBuffer)
	mpstop()
}

func BenchmarkBaselineConnWriteSideLargeBuffer(b *testing.B) {
	c1, c2, mpstop, err := mp()
	if err != nil {
		b.Errorf("scaffold")
	}
	benchmarkConnWriteSide(b, c1, c2, largeBuffer)
	mpstop()
}

func benchmarkMuxConnReadSide(b *testing.B, bufferSize int) {
	benchmarkMuxConn(b, bufferSize, benchmarkConnReadSide)
}

func benchmarkMuxConnWriteSide(b *testing.B, bufferSize int) {
	benchmarkMuxConn(b, bufferSize, benchmarkConnWriteSide)
}

func benchmarkMuxConn(b *testing.B, bufferSize int, bench func(*testing.B, net.Conn, net.Conn, int)) {
	x1, x2, mpstop, err := mp()
	if err != nil {
		b.Errorf("scaffold")
	}
	m1, err := NewMuxConn(x1, true)
	if err != nil {
		b.Errorf("new mux conn 1")
	}
	m2, err := NewMuxConn(x2, false)
	if err != nil {
		b.Errorf("new mux conn 2")
	}
	id, c1, err := m1.Mux()
	if err != nil {
		b.Errorf("new mux conn 3")
	}
	c2, err := m2.DialConn(id)
	if err != nil {
		b.Errorf("dial conn 3")
	}

	bench(b, c1, c2, bufferSize)
	_ = m1.Close()
	_ = m2.Close()
	mpstop()
}

func benchmarkConnReadSide(b *testing.B, c1, c2 net.Conn, bufferSize int) {
	quitCh := make(chan struct{})
	go func() {
		want := make([]byte, bufferSize)
		rand.New(rand.NewSource(0)).Read(want)
		for {
			rd := bytes.NewReader(want)
			_, _ = io.Copy(struct{ io.Writer }{c1}, struct{ io.Reader }{rd})
			select {
			case <-quitCh:
				return
			default:
			}
		}
	}()

	temp := make([]byte, bufferSize)
	rand.New(rand.NewSource(0)).Read(temp)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c2.Read(temp)
	}

	close(quitCh)
	_ = c1.Close()
	_ = c2.Close()
}

func benchmarkConnWriteSide(b *testing.B, c1, c2 net.Conn, bufferSize int) {
	quitCh := make(chan struct{})
	go func() {
		for {
			_, _ = io.Copy(struct{ io.Writer }{io.Discard}, struct{ io.Reader }{c2})
			select {
			case <-quitCh:
				return
			default:
			}
		}
	}()

	temp := make([]byte, bufferSize)
	rand.New(rand.NewSource(0)).Read(temp)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c1.Write(temp)
	}

	close(quitCh)
	_ = c1.Close()
	_ = c2.Close()
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
