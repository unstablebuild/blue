package gps

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/ernestrc/blue/rpc"
	"github.com/golang/protobuf/proto"
	"github.com/pion/dtls/v2"
	log "github.com/sirupsen/logrus"
)

const connectTimeout = 10 * time.Second

type dtlsSender struct {
	conn   net.Conn
	addr   *net.UDPAddr
	config dtls.Config
	ch     chan error
}

// NewDTLSSender returns a Sender that transmits the GPS location at the GPS
// server at ip and port, with the given DTLS configuration.
func NewDTLSSender(ip string, port int, config dtls.Config) (Sender, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, errors.New("invalid IP address")
	}

	s := new(dtlsSender)
	s.addr = &net.UDPAddr{IP: parsedIP, Port: port}
	s.config = config
	s.ch = make(chan error)

	return s, nil
}

func (s *dtlsSender) Send(ctx context.Context, pos Coordinates) error {
	if s.conn == nil {
		ctx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()

		var err error
		s.conn, err = dtls.DialWithContext(ctx, "udp", s.addr, &s.config)
		if err != nil {
			s.conn = nil /* on err conn might not be nil */
			return err
		}

		log.Debugf("Connected to DTLS server %+v", s.addr)
	}
	b, err := proto.Marshal(&rpc.Coordinates{
		Latitude:  float32(pos.Latitude),
		Longitude: float32(pos.Longitude),
		Altitude:  float32(pos.Altitude),
	})
	if err != nil {
		return err
	}

	go func(conn net.Conn) {
		_, err := conn.Write(b)
		s.ch <- err
	}(s.conn)

	select {
	case <-ctx.Done():
		s.conn.SetWriteDeadline(time.Now())
		return <-s.ch
	case err := <-s.ch:
		return err
	}
}

func (s *dtlsSender) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	conn := s.conn
	s.conn = nil
	return conn.Close()
}
