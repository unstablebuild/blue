package main

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/gps"
	"github.com/jacobsa/go-serial/serial"
)

// build with
// GOOS=linux GOARCH=arm GOARM=5 go build
func main() {
	const serialDevice = "ttyS0"
	options := serial.OpenOptions{
		PortName:        fmt.Sprintf("/dev/%s", serialDevice),
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}
	pos, err := gps.NewSerialDevicePositioner(options)
	if err != nil {
		log.Fatalf("failed to create serial device GPS positioner: %s\n", err)
	}
	pos = gps.WithLogging(pos, "serial-device-"+serialDevice)
	defer pos.Close()

	for {
		ctx := context.Background()
		p, err := pos.Position(ctx)
		if err != nil {
			continue // logged by logging Positioner
		}
		log.Infof("found gps position: %+v", p)

		// TODO push gps location to server
	}
}
