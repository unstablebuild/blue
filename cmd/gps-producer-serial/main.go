package main

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/gps"
	"github.com/ernestrc/blue/logging"
	"github.com/jacobsa/go-serial/serial"
)

const cadence = 1 * time.Second
const serialDevice = "ttyS0"
const ip = "127.0.0.1"

var opts = serial.OpenOptions{
	PortName:        fmt.Sprintf("/dev/%s", serialDevice),
	BaudRate:        9600,
	DataBits:        8,
	StopBits:        1,
	MinimumReadSize: 4,
}

// build with
// GOOS=linux GOARCH=arm GOARM=5 go build
func main() {
	logging.SetDefaults(true)

	pos, err := gps.NewSerialDevicePositioner(opts)
	if err != nil {
		log.Fatalf("failed to create serial device GPS positioner: %s\n", err)
	}
	pos = gps.WithLogging(pos, "serial-device-"+serialDevice)
	defer pos.Close()

	c, err := gps.NewDTLSSender(ip, gps.DefaultPort)
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
		log.Infof("found gps position: %+v", p)

		err = c.Send(p)
		if err != nil {
			log.Warnf("failed to send GPS position to server: %v", err)
		}

		time.Sleep(cadence)
	}
}
