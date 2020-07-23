package gps

import (
	"errors"
	"net"

	"github.com/ernestrc/blue/rpc"
	"github.com/golang/protobuf/proto"
	"github.com/pion/dtls/v2"
	log "github.com/sirupsen/logrus"
)

type dtlsSender struct {
	conn *dtls.Conn
}

func NewDTLSSender(ip string, port int) (Sender, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, errors.New("invalid IP address")
	}

	s := new(dtlsSender)
	addr := &net.UDPAddr{IP: parsedIP, Port: port}
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			// fmt.Printf("Server's hint: %s \n", hint)
			return []byte{0xAB, 0xC1, 0x23}, nil
		},
		PSKIdentityHint:      []byte("Pion DTLS Server"),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	var err error
	s.conn, err = dtls.Dial("udp", addr, config)
	if err != nil {
		return nil, err
	}

	log.Infof("Connected to DTLS server %+v", addr)

	return s, nil
}

func (c *dtlsSender) Send(pos Coordinates) error {
	b, err := proto.Marshal(&rpc.Coordinates{
		Latitude:  float32(pos.Latitude),
		Longitude: float32(pos.Longitude),
		Altitude:  float32(pos.Altitude),
	})
	if err != nil {
		return err
	}

	_, err = c.conn.Write(b)
	return err
}

func (c *dtlsSender) Close() error {
	return c.conn.Close()
}
