package main

import (
	"context"
	"time"

	"github.com/pion/dtls/v2"
	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/gps"
	"github.com/ernestrc/blue/logging"
)

const (
	ip      = "127.0.0.1"
	cadence = 1 * time.Second
)

var config = dtls.Config{
	PSK: func(hint []byte) ([]byte, error) {
		// fmt.Printf("Server's hint: %s \n", hint)
		return []byte{0xAB, 0xC1, 0x23}, nil
	},
	PSKIdentityHint:      []byte("Pion DTLS Server"),
	CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
	ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
}

func main() {
	logging.SetDefaults(true)

	pos := gps.NewStaticPositioner(gps.Coordinates{
		Latitude:  3.4,
		Longitude: 4.5,
		Altitude:  11.0,
	})
	pos = gps.WithLoggingPositioner(pos, "static")
	defer pos.Close()

	c, err := gps.NewDTLSSender(ip, gps.DefaultPort, config)
	if err != nil {
		log.Fatalf("failed to create gps client: %v", err)
	}
	defer c.Close()

	for {
		p, err := pos.Position(context.Background())
		if err != nil {
			continue // logged by logging Positioner
		}

		err = c.Send(context.Background(), p)
		if err != nil {
			log.Warnf("failed to send GPS position to server: %v", err)
		}

		time.Sleep(cadence)
	}
}
