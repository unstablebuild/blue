package proto

import (
	"context"
	"io"
	"math"
	"net"
	"sync"
	"time"

	multierr "github.com/ernestrc/go-multierror"
)

// ChanConn returns an implementation of net.Conn backed by two channels.
func ChanConn(local, remote net.Addr, read <-chan []byte, write chan<- []byte) net.Conn {
	ret := &chanConn{
		local:            local,
		remote:           remote,
		read:             read,
		write:            write,
		readDeadline:     time.Now().Add(math.MaxInt64),
		writeDeadline:    time.Now().Add(math.MaxInt64),
		newReadDeadline:  make(chan struct{}),
		newWriteDeadline: make(chan struct{}),
	}
	ret.closeCtx, ret.cancelCloseCtx = context.WithCancel(context.Background())
	return ret
}

// satisfy net.Error
var errTimeout error = &timeoutError{}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

func (e *timeoutError) Is(err error) bool {
	return err == context.DeadlineExceeded
}

type chanConn struct {
	mu            sync.Mutex
	local, remote net.Addr
	read          <-chan []byte
	write         chan<- []byte
	pending       []byte

	closeCtx       context.Context
	cancelCloseCtx func()

	// wait for all writes to return
	// before closing write chan
	writeCloseWg     sync.WaitGroup
	writeDeadline    time.Time
	readDeadline     time.Time
	newReadDeadline  chan struct{}
	newWriteDeadline chan struct{}
}

func (c *chanConn) Read(b []byte) (n int, err error) {
	c.mu.Lock()
	now := time.Now()
	deadline := c.readDeadline
	deadlineChan := c.newReadDeadline
	if now.After(deadline) {
		err = &timeoutError{}
		c.mu.Unlock()
		return
	}

	if len(c.pending) > 0 {
		n = copy(b, c.pending)
		c.pending = c.pending[n:]
	}
	copied := len(b) == n
	b = b[n:]
	closed := c.cancelCloseCtx == nil

	c.mu.Unlock()

	if closed {
		err = net.ErrClosed
		return
	}

	if copied {
		return
	}

	timer := time.NewTimer(deadline.Sub(now))
	for {
		select {
		case <-c.closeCtx.Done():
			err = net.ErrClosed
			return
		case <-deadlineChan:
			c.mu.Lock()
			deadline = c.readDeadline
			deadlineChan = c.newReadDeadline
			c.mu.Unlock()
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(deadline.Sub(time.Now()))
		case <-timer.C:
			err = &timeoutError{}
			return
		case bytes, ok := <-c.read:
			if !ok {
				err = io.EOF
				return
			}

			c.mu.Lock()
			defer c.mu.Unlock()

			m := copy(b, bytes)
			n += m
			if len(bytes) == m {
				return
			}
			c.pending = append(c.pending, bytes[m:]...)
			return
		}
	}
}

func (c *chanConn) Write(b []byte) (n int, err error) {
	c.mu.Lock()
	now := time.Now()
	deadline := c.writeDeadline
	deadlineChan := c.newWriteDeadline
	closed := c.cancelCloseCtx == nil
	c.writeCloseWg.Add(1)
	c.mu.Unlock()
	defer c.writeCloseWg.Done()

	if now.After(deadline) {
		err = &timeoutError{}
		return
	}

	if closed {
		err = net.ErrClosed
		return
	}

	temp := make([]byte, len(b))
	copy(temp, b)

	timer := time.NewTimer(deadline.Sub(now))
	for {
		select {
		case <-deadlineChan:
			// if we are closing and at the same time a deadline expires:
			// if deadline ch has already been closed then close ctx also
			// must have been closed so this protects against this select
			// matching against deadlineChan rather than closeCtx.Done for
			// a slow system and causing a deadlock between Lock below
			// and wg.Wait in close.
			select {
			case <-c.closeCtx.Done():
				err = net.ErrClosed
				return
			default:
			}
			c.mu.Lock()
			deadline = c.writeDeadline
			deadlineChan = c.newWriteDeadline
			c.mu.Unlock()
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(deadline.Sub(time.Now()))
		case <-c.closeCtx.Done():
			err = net.ErrClosed
			return
		case <-timer.C:
			err = &timeoutError{}
			return
		case c.write <- temp:
			return len(temp), nil
		}
	}
}

func (c *chanConn) LocalAddr() net.Addr {
	return c.local
}

func (c *chanConn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *chanConn) SetDeadline(t time.Time) (ret error) {
	if err := c.SetReadDeadline(t); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := c.SetWriteDeadline(t); err != nil {
		ret = multierr.Append(ret, err)
	}
	return
}

func (c *chanConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.newReadDeadline)
	if t == (time.Time{}) {
		t = time.Now().Add(math.MaxInt64)
	}
	c.readDeadline = t
	c.newReadDeadline = make(chan struct{})
	return nil
}

func (c *chanConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.newWriteDeadline)
	if t == (time.Time{}) {
		t = time.Now().Add(math.MaxInt64)
	}
	c.writeDeadline = t
	c.newWriteDeadline = make(chan struct{})
	return nil
}

func (c *chanConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// avoid double close of write channel
	if c.cancelCloseCtx == nil {
		return nil
	}
	c.cancelCloseCtx()
	c.cancelCloseCtx = nil

	c.writeCloseWg.Wait()
	close(c.write)

	return nil
}
