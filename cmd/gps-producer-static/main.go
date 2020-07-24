package main

import (
	"time"

	"github.com/pion/dtls/v2"
	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/gps"
	"github.com/ernestrc/blue/logging"
)

const (
	ip      = "127.0.0.1"
	cadence = 5 * time.Second
)

var config = dtls.Config{
	PSK: func(hint []byte) ([]byte, error) {
		return []byte{0xAB, 0xC1, 0x23}, nil
	},
	PSKIdentityHint:      []byte("Pion DTLS Server"),
	CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
	ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
}

func main() {
	logging.SetDefaults(true)

	p := gps.NewStaticPositioner(gps.Coordinates{
		Latitude:  3.4,
		Longitude: 4.5,
		Altitude:  11.0,
	})
	p = gps.WithLoggingPositioner(p, "static")
	defer p.Close()

	s, err := gps.NewDTLSSender(ip, gps.DefaultPort, config)
	if err != nil {
		log.Fatalf("failed to create gps client: %v", err)
	}
	defer s.Close()

	gps.SendPositionAtCadence(p, s, cadence)
}
