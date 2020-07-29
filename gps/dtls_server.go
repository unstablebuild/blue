package gps

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ernestrc/blue/datastore"
	"github.com/ernestrc/blue/rpc"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"

	"github.com/pion/dtls/v2"
)

// DefaultPort is the default port used by GPS servers and clients.
const DefaultPort = 4677

const maxDatagramSize = 8192

// DTLSServer is a DTLS GPS server.
type DTLSServer struct {
	cancelCtx context.Context
	cancel    func()
	listener  net.Listener
	addr      *net.UDPAddr

	receivers []Receiver
	conns     map[ConnectionMetadata]net.Conn
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

	s.conns = make(map[ConnectionMetadata]net.Conn)
	s.receivers = receivers

	s.cancelCtx, s.cancel = context.WithCancel(context.Background())

	config.ConnectContextMaker = func() (context.Context, func()) {
		return context.WithTimeout(s.cancelCtx, 30*time.Second)
	}

	s.listener, err = dtls.Listen("udp", s.addr, &config)
	if err != nil {
		s.cancel()
		return nil, err
	}

	return s, nil
}

func (s *DTLSServer) connectionRemove(meta ConnectionMetadata, conn *dtls.Conn) {
	s.lock.Lock()
	delete(s.conns, meta)
	err := conn.Close()
	s.lock.Unlock()

	if err != nil {
		log.Warnf("Failed to disconnect %v: %s", conn.RemoteAddr(), err)
	} else {
		log.Debugf("Disconnected %v", conn.RemoteAddr())
		for _, r := range s.receivers {
			r.OnClose(meta)
		}
	}
}

func (s *DTLSServer) connectionRead(meta ConnectionMetadata, conn *dtls.Conn) {
	log.Debugf("Reading messages from %v", meta)

	b := make([]byte, maxDatagramSize)
	for {
		n, err := conn.Read(b)
		if err != nil {
			log.Errorf("failed to read: %v", err)
			s.connectionRemove(meta, conn)
			return
		}

		in := rpc.Coordinates{}
		err = proto.Unmarshal(b[:n], &in)
		if err != nil {
			log.Errorf("failed to unmarshal coordinates: %v", err)
			s.connectionRemove(meta, conn)
			return
		}

		for _, r := range s.receivers {
			r.Receive(meta, Coordinates{
				DeviceID:  in.GetDeviceID(),
				Altitude:  in.GetAltitude(),
				Latitude:  in.GetLatitude(),
				Longitude: in.GetLongitude(),
				Time:      datastore.ProtoTimeToStd(in.GetTime()),
			})
		}
	}
}

func (s *DTLSServer) serveOne() error {
	conn, err := s.listener.Accept()
	if err != nil {
		return err
	}

	meta := ConnectionMetadata{
		RemoteAddr: conn.RemoteAddr(),
		LocalAddr:  conn.LocalAddr(),
	}

	log.Debugf("Accepted new connection: %v", meta)

	s.lock.Lock()
	s.conns[meta] = conn
	s.lock.Unlock()

	for _, r := range s.receivers {
		r.OnOpen(meta)
	}

	go s.connectionRead(meta, conn.(*dtls.Conn))

	return nil
}

// Serve starts accepting new connections and reading GPS coordinates.
func (s *DTLSServer) Serve() error {
	log.Debugf("GPS DTLSServer listening on udp addr %v", s.addr)

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

	for meta, c := range s.conns {
		for _, r := range s.receivers {
			r.OnClose(meta)
		}
		_ = c.Close()
	}
	s.conns = nil
	return s.listener.Close()
}
