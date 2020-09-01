package gps

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging/trace"
	"github.com/ernestrc/blue/rpc"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"

	"github.com/pion/dtls/v2"
)

// DefaultPort is the default port used by GPS servers and clients.
const (
	DefaultPort       = 4677
	maxDatagramSize   = 8192
	receiveTimeout    = 3 * time.Second
	sendAckTimeout    = 500 * time.Millisecond
	ackCadence        = 30 * time.Second
	serverReadTimeout = 2 * time.Minute
)

// connectionMetadata represents the connection metadata of a connection-based
// GPS receiver.
type connectionMetadata struct {
	RemoteAddr net.Addr
	LocalAddr  net.Addr
}

// DTLSServer is a DTLS GPS server.
type DTLSServer struct {
	cancelCtx context.Context
	cancel    func()
	listener  net.Listener
	addr      *net.UDPAddr

	receivers []Receiver
	conns     map[connectionMetadata]net.Conn
	lock      sync.RWMutex
}

// NewDTLSServer allocates storage for a new instance of DTLSServer and initializes it.
// dtls.Config.ConnectContextMaker is overriden by this constructor to set
// a connect timeout of 30s. GPS Coordinates received are delegated to receivers.
func NewDTLSServer(
	host string, port int, config dtls.Config, receivers ...Receiver,
) (*DTLSServer, error) {
	s := new(DTLSServer)

	var err error
	s.addr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}

	s.conns = make(map[connectionMetadata]net.Conn)
	s.receivers = receivers

	s.cancelCtx, s.cancel = context.WithCancel(context.Background())

	config.ConnectContextMaker = func() (context.Context, func()) {
		return context.WithTimeout(s.cancelCtx, connectTimeout)
	}

	s.listener, err = dtls.Listen("udp", s.addr, &config)
	if err != nil {
		s.cancel()
		return nil, err
	}

	return s, nil
}

func (s *DTLSServer) connectionRemove(meta connectionMetadata, conn *dtls.Conn) {
	s.lock.Lock()
	delete(s.conns, meta)
	err := conn.Close()
	s.lock.Unlock()

	if err != nil {
		log.Warnf("Failed to disconnect %v: %s", conn.RemoteAddr(), err)
	} else {
		log.Debugf("Disconnected %v", conn.RemoteAddr())
	}
}

func (s *DTLSServer) receiveWithTimeout(
	ctx context.Context, r Receiver, in rpc.Coordinates,
) {
	ctx, cancel := context.WithTimeout(ctx, receiveTimeout)
	defer cancel()

	r.Receive(ctx, Coordinates{
		DeviceID:  in.GetDeviceID(),
		Altitude:  in.GetAltitude(),
		Latitude:  in.GetLatitude(),
		Longitude: in.GetLongitude(),
		UnixTime:  in.GetUnixTime(),
	})
}

func (s *DTLSServer) sendAck(conn *dtls.Conn, ackErrChan chan error) error {
	ctx, cancel := context.WithTimeout(context.Background(), sendAckTimeout)
	defer cancel()

	b, err := proto.Marshal(&rpc.Ack{
		UnixTime: time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	return sendBytesConnWait(ctx, b, conn, ackErrChan)
}

func (s *DTLSServer) connectionRead(meta connectionMetadata, conn *dtls.Conn) {
	log.Debugf("Reading messages from %v", meta)

	ackTimer := time.NewTimer(ackCadence)
	ackErrChan := make(chan error)
	errCh := make(chan error)
	b := make([]byte, maxDatagramSize)

	for {
		in := rpc.Coordinates{}
		ctx := trace.NewContext(context.Background(), trace.New())
		ctx = withConnectionMeta(ctx, meta)
		readCtx, cancel := context.WithTimeout(ctx, serverReadTimeout)
		readBytesConn(readCtx, b, conn, &in, errCh)

		var err error
		select {
		case err = <-errCh:
		case <-readCtx.Done():
			err = <-errCh
		case <-ackTimer.C:
			err = s.sendAck(conn, ackErrChan)
			ackTimer.Reset(ackCadence)
			if err == nil {
				// wait until read is drained
				select {
				case <-readCtx.Done():
					err = <-errCh
				case err = <-errCh:
				}
			}
		}
		cancel()
		if err != nil {
			log.Errorf("failed to read %v: %v", meta, err)
			conn.SetReadDeadline(time.Now())
			s.connectionRemove(meta, conn)
			return
		}

		for _, r := range s.receivers {
			s.receiveWithTimeout(ctx, r, in)
		}
	}
}

func (s *DTLSServer) serveOne() error {
	conn, err := s.listener.Accept()
	if err != nil {
		return err
	}

	meta := connectionMetadata{
		RemoteAddr: conn.RemoteAddr(),
		LocalAddr:  conn.LocalAddr(),
	}

	log.Debugf("Accepted new connection: %v", meta)

	s.lock.Lock()
	s.conns[meta] = conn
	s.lock.Unlock()

	go s.connectionRead(meta, conn.(*dtls.Conn))

	return nil
}

// Serve starts accepting new connections and reading GPS coordinates.
func (s *DTLSServer) Serve() error {
	log.Infof("GPS DTLSServer listening on udp addr %v", s.addr)

	for {
		err := s.serveOne()
		switch e := err.(type) {
		case nil:
		case (net.Error):
			if e.Temporary() {
				log.Warn(err)
				continue
			}
			return err
		default:
			return err
		}
	}
}

// Addr returns the address that this instance of DTLSServer is listening on.
func (s *DTLSServer) Addr() *net.UDPAddr {
	if s.listener == nil {
		panic("called Addr() before ServeAndListen was called")
	}
	return s.listener.Addr().(*net.UDPAddr)
}

// Close closes all resources associated with this instance of DTLSServer.
func (s *DTLSServer) Close() error {
	s.cancel()
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, c := range s.conns {
		_ = c.Close()
	}
	s.conns = nil
	return s.listener.Close()
}
