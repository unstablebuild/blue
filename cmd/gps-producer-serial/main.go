package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pion/dtls/v2"
	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/gps"
	"github.com/ernestrc/blue/logging"
	"github.com/jacobsa/go-serial/serial"
)

const (
	cadence      = 1 * time.Second
	serialDevice = "ttyS0"
	ip           = "127.0.0.1"
)

var (
	serialOpts = serial.OpenOptions{
		PortName:        fmt.Sprintf("/dev/%s", serialDevice),
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	dtlsConfig = dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			// fmt.Printf("Server's hint: %s \n", hint)
			return []byte{0xAB, 0xC1, 0x23}, nil
		},
		PSKIdentityHint:      []byte("Pion DTLS Server"),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}
)

// build with
// GOOS=linux GOARCH=arm GOARM=5 go build
func main() {
	logging.SetDefaults(true)

	pos, err := gps.NewSerialDevicePositioner(serialOpts)
	if err != nil {
		log.Fatalf("failed to create serial device GPS positioner: %s\n", err)
	}
	pos = gps.WithLoggingPositioner(pos, "serial-device-"+serialDevice)
	defer pos.Close()

	c, err := gps.NewDTLSSender(ip, gps.DefaultPort, dtlsConfig)
	if err != nil {
		log.Fatalf("failed to create gps client: %v", err)
	}
	defer c.Close()

	for {
		ctx := context.Background()
		p, err := pos.Position(ctx)
		if err != nil {
			continue // logged by logging Positioner
		}

		err = c.Send(ctx, p)
		if err != nil {
			log.Warnf("failed to send GPS position to server: %v", err)
		}

		// TODO use timer to wake up instead of sleep
		time.Sleep(cadence)
	}
}
