package gps

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ernestrc/blue/gps/rpc"
	"github.com/ernestrc/blue/logging"
	"github.com/pion/dtls/v2"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

const (
	connectTimeout    = 5 * time.Second
	clientReadTimeout = 3 * ackCadence
)

type dtlsSender struct {
	lock     sync.Mutex
	conn     *dtls.Conn
	addr     *net.UDPAddr
	config   dtls.Config
	ch       chan error
	quitChan chan struct{}
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

func sendPosition(
	ctx context.Context, conn *dtls.Conn, pos Coordinates, ch chan error,
) error {
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

	return sendBytesConnWait(ctx, b, conn, ch)
}

func (s *dtlsSender) makeConn(ctx context.Context) (*dtls.Conn, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.conn == nil {
		ctx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()

		var err error
		s.conn, err = dtls.DialWithContext(ctx, "udp", s.addr, &s.config)
		if err != nil {
			s.conn = nil /* on err conn might not be nil */
			return nil, err
		}

		s.quitChan = make(chan struct{})
		go s.monitorConn(s.quitChan)

		log.Infof("Connected to DTLS server %+v", s.addr)
	}

	return s.conn, nil
}

func (s *dtlsSender) rmConn() *dtls.Conn {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s == nil || s.conn == nil {
		return nil
	}
	conn := s.conn
	s.conn = nil
	close(s.quitChan)

	log.Infof("Removed connection to server: %v", conn.RemoteAddr().String())

	return conn
}

func (s *dtlsSender) monitorConn(quitChan chan struct{}) {
	// if we don't hear from server in N ack periods,
	// close and rm connection to force re-connection
	b := make([]byte, maxDatagramSize)
	errCh := make(chan error)
	for {
		select {
		case <-quitChan:
			return
		default:
		}

		s.lock.Lock()
		conn := s.conn
		s.lock.Unlock()
		if conn == nil {
			return
		}

		in := rpc.Ack{}
		ctx := context.Background()
		err := readBytesConnWait(ctx, clientReadTimeout, b, conn, &in, errCh)
		if err != nil {
			log.Warningf("monitor dtls conn: failed to read from conn: %v", err)
			s.rmConn()
			return
		}

		log.WithFields(log.Fields{
			"RemoteAddr":        conn.RemoteAddr().String(),
			logging.KeyCallType: "MonitorAck",
			logging.KeyStep:     logging.ValueStepSuccess,
		}).Trace()
	}
}

func (s *dtlsSender) Send(ctx context.Context, pos Coordinates) error {
	conn, err := s.makeConn(ctx)
	if err != nil {
		return err
	}
	return sendPosition(ctx, conn, pos, s.ch)
}

func (s *dtlsSender) Close() error {
	conn := s.rmConn()
	if conn == nil {
		return nil
	}

	return conn.Close()
}
