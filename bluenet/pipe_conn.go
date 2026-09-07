// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bluenet

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

// StdioConn is a specialized version of PipeConn which uses
// os.Stdin as reader and os.Stdout as the writer.
//
// Calls to SetDeadline, SetReadDeadline and SetWriteDeadline
// always fail, as stdio files are not files created via Pipe.
func StdioConn() net.Conn {
	return &pipesConn{
		local:  newStdinAddr("stdio"),
		remote: newStdinAddr("stdio"),
		r:      os.Stdin,
		w:      os.Stdout,
	}
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
