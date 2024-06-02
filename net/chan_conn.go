//nolint:gosimple
package net

import (
	"context"
	"io"
	"math"
	"net"
	"sync"
	"time"

	multierr "github.com/ernestrc/go-multierror"
)

// ReadResult represents the result of a net.Conn Read. Ch is used
// to notify the write-side that it's safe for Write to return
// by means of closing the channel, so there's no need to drain if
// an ack is not required.
type ReadResult struct {
	Data  []byte
	Error error
	Ch    chan struct{}
}

// ChanConn returns an implementation of net.Conn backed by two channels.
func ChanConn(local, remote net.Addr, read <-chan ReadResult, write chan<- ReadResult) net.Conn {
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

type chanConn struct {
	mu            sync.Mutex
	local, remote net.Addr
	read          <-chan ReadResult
	write         chan<- ReadResult
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
		case result, ok := <-c.read:
			if !ok {
				err = io.EOF
				return
			}
			close(result.Ch)

			if result.Error != nil {
				err = result.Error
				return
			}

			c.mu.Lock()
			defer c.mu.Unlock()

			m := copy(b, result.Data)
			n += m
			if len(result.Data) == m {
				return
			}
			c.pending = append(c.pending, result.Data[m:]...)
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
		res := ReadResult{Data: temp, Ch: make(chan struct{})}
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
		case c.write <- res:
			for {
				select {
				case <-res.Ch:
					return len(temp), nil
				case <-deadlineChan:
					c.mu.Lock()
					deadline = c.writeDeadline
					deadlineChan = c.newWriteDeadline
					c.mu.Unlock()
					if !timer.Stop() {
						<-timer.C
					}
					timer.Reset(deadline.Sub(time.Now()))
					continue
				case <-c.closeCtx.Done():
					err = net.ErrClosed
					return
				case <-timer.C:
					err = &timeoutError{}
					return
				}
			}
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
