package gps

import (
	"context"
	"fmt"
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
// server at host and port, with the given DTLS configuration.
func NewDTLSSender(host string, port int, config dtls.Config) (Sender, error) {
	s := new(dtlsSender)

	var err error
	s.addr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}

	s.config = config
	s.ch = make(chan error)

	return s, nil
}

func (s *dtlsSender) sendBytes(ctx context.Context, b []byte) error {
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

func (s *dtlsSender) sendPosition(ctx context.Context, pos Coordinates) error {
	b, err := proto.Marshal(&rpc.Coordinates{
		DeviceID:  pos.DeviceID,
		Latitude:  float32(pos.Latitude),
		Longitude: float32(pos.Longitude),
		Altitude:  float32(pos.Altitude),
		UnixTime:  pos.UnixTime,
	})
	if err != nil {
		return err
	}

	return s.sendBytes(ctx, b)
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
	return s.sendPosition(ctx, pos)
}

func (s *dtlsSender) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	conn := s.conn
	s.conn = nil
	return conn.Close()
}
