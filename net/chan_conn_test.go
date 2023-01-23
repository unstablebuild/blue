package proto

import (
	"net"
	"testing"

	"golang.org/x/net/nettest"
)

func TestChanConn(t *testing.T) {
	nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
		addr1, addr2 := &net.UDPAddr{Port: 1}, &net.UDPAddr{Port: 2}
		ch1, ch2 := make(chan []byte), make(chan []byte)
		c1 = ChanConn(addr1, addr2, ch1, ch2)
		c2 = ChanConn(addr2, addr1, ch2, ch1)
		stop = func() {
			_ = c1.Close()
			_ = c2.Close()
		}
		return
	})
}
