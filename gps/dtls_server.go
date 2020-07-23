package gps

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/ernestrc/blue/rpc"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"

	"github.com/pion/dtls/v2"
)

const maxDatagramSize = 8192

// Server is a DTLS GPS server.
type Server struct {
	cancelCtx context.Context
	cancel    func()
	listener  net.Listener
	addr      *net.UDPAddr

	conns map[string]net.Conn
	lock  sync.RWMutex
}

// NewServer allocates storage for a new instance of Server and initializes it.
func NewServer(ip string, port int) (*Server, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, errors.New("invalid IP address")
	}

	s := new(Server)
	s.conns = make(map[string]net.Conn)
	s.addr = &net.UDPAddr{IP: parsedIP, Port: port}

	s.cancelCtx, s.cancel = context.WithCancel(context.Background())

	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			// fmt.Printf("Client's hint: %s \n", hint)
			return []byte{0xAB, 0xC1, 0x23}, nil
		},
		PSKIdentityHint:      []byte("Pion DTLS Client"),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
		// Create timeout context for accepted connection.
		ConnectContextMaker: func() (context.Context, func()) {
			return context.WithTimeout(s.cancelCtx, 30*time.Second)
		},
	}

	var err error
	s.listener, err = dtls.Listen("udp", s.addr, config)
	if err != nil {
		s.cancel()
		return nil, err
	}

	return s, nil
}

func (s *Server) connectionRemove(conn *dtls.Conn) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.conns, conn.RemoteAddr().String())
	err := conn.Close()
	if err != nil {
		log.Warn("Failed to disconnect", conn.RemoteAddr(), err)
	} else {
		log.Info("Disconnected ", conn.RemoteAddr())
	}
}

func (s *Server) connectionRead(conn *dtls.Conn) {
	s.lock.Lock()
	s.conns[conn.RemoteAddr().String()] = conn
	s.lock.Unlock()

	log.Debugf("Reading messages from %s", conn.RemoteAddr())

	b := make([]byte, maxDatagramSize)
	for {
		n, err := conn.Read(b)
		if err != nil {
			log.Errorf("failed to read: %v", err)
			s.connectionRemove(conn)
			return
		}

		in := rpc.Coordinates{}
		err = proto.Unmarshal(b[:n], &in)
		if err != nil {
			log.Errorf("failed to unmarshal coordinates: %v", err)
			s.connectionRemove(conn)
			return
		}

		// TODO handle gps position through handlers?
		log.Infof("received GPS position: %v", in)
	}
}

// ListenAndServe starts accepting new connections and reading GPS coordinates.
func (s *Server) ListenAndServe() error {
	log.Infof("GPS Server listening on udp addr %v", s.addr)

	for {
		conn, err := s.listener.Accept()
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

		log.Infof("Accepted new connection: %s", conn.RemoteAddr())

		go s.connectionRead(conn.(*dtls.Conn))
	}
}

// Close closes all resources associated with this instance of Server.
func (s *Server) Close() error {
	s.cancel()
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, c := range s.conns {
		_ = c.Close()
	}
	s.conns = nil
	return s.listener.Close()
}
