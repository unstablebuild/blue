package net

import (
	"io/fs"
	"net"
	"os"
	"time"

	multierr "github.com/ernestrc/go-multierror"
)

// PipeConn wraps a pair of Files representing the read
// and write side of two connected pipes and returns a net.Conn
//
// It is up to the caller to ensure that r and w and indeed the
// read-side and the write-side of a pipe. If r or w are not a pipe
// or the system doesn't support setting deadlines on files,
// then this method returns an error.
//
// A common use for this method is to wrap os.Stdin and os.Stdout
// to use as a net.Conn.
func PipeConn(r *os.File, w *os.File) (net.Conn, error) {
	for _, f := range [2]*os.File{r, w} {
		if err := f.SetDeadline(time.Time{}); err != nil {
			return nil, err
		}
	}
	return &pipesConn{
		local:  newStdinAddr(r.Name()),
		remote: newStdinAddr(w.Name()),
		r:      r,
		w:      w,
	}, nil
}

type pipesConn struct {
	r      *os.File
	w      *os.File
	local  *stdinAddr
	remote *stdinAddr
}

func (p *pipesConn) Read(b []byte) (n int, err error) {
	n, err = p.r.Read(b)
	if err != nil {
		err = mapError(err)
	}
	return
}

func (p *pipesConn) Write(b []byte) (n int, err error) {
	n, err = p.w.Write(b)
	if err != nil {
		err = mapError(err)
	}
	return
}

func mapError(err error) error {
	if ferr, ok := err.(*fs.PathError); ok && ferr.Timeout() {
		return &timeoutError{} // satisfy net.Error
	}
	return err
}

func (p *pipesConn) Close() (ret error) {
	for _, f := range [2]*os.File{p.w, p.r} {
		if err := f.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return ret
}

func (p *pipesConn) LocalAddr() net.Addr {
	return p.local
}

func (p *pipesConn) RemoteAddr() net.Addr {
	return p.remote
}

func (p *pipesConn) SetDeadline(t time.Time) (ret error) {
	if err := p.SetReadDeadline(t); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := p.SetWriteDeadline(t); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

func (p *pipesConn) SetReadDeadline(t time.Time) error {
	return p.r.SetReadDeadline(t)
}

func (p *pipesConn) SetWriteDeadline(t time.Time) error {
	return p.w.SetWriteDeadline(t)
}

type stdinAddr struct {
	s string
}

func newStdinAddr(s string) *stdinAddr {
	return &stdinAddr{s}
}

func (a *stdinAddr) Network() string {
	return "pipe"
}

func (a *stdinAddr) String() string {
	return a.s
}
