package main

import (
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

	p, err := gps.NewSerialDevicePositioner(serialOpts)
	if err != nil {
		log.Fatalf("failed to create serial device GPS positioner: %s\n", err)
	}
	p = gps.WithLoggingPositioner(p, "serial-device-"+serialDevice)
	defer p.Close()

	s, err := gps.NewDTLSSender(ip, gps.DefaultPort, dtlsConfig)
	if err != nil {
		log.Fatalf("failed to create gps client: %v", err)
	}
	defer s.Close()

	gps.SendPositionAtCadence(p, s, cadence)
}
