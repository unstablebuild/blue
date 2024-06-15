// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
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
