package net

import (
	"net"
	"os"
	"testing"

	"golang.org/x/net/nettest"
)

func TestPipeConn(t *testing.T) {
	nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
		var r1, w1, r2, w2 *os.File
		r1, w2, err = os.Pipe()
		if err != nil {
			return
		}
		r2, w1, err = os.Pipe()
		if err != nil {
			return
		}
		c1, err = PipeConn(r1, w1)
		if err != nil {
			return
		}
		c2, err = PipeConn(r2, w2)
		if err != nil {
			return
		}
		stop = func() {
			_ = r1.Close()
			_ = r2.Close()
			_ = w1.Close()
			_ = w2.Close()
		}
		return
	})
}
