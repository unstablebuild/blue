package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/gps"
	"github.com/ernestrc/blue/logging"
)

const ip = "127.0.0.1"
const cadence = 1 * time.Second

func main() {
	logging.SetDefaults(true)

	pos := gps.NewStaticPositioner(gps.Coordinates{
		Latitude:  3.4,
		Longitude: 4.5,
		Altitude:  11.0,
	})
	pos = gps.WithLogging(pos, "static")
	defer pos.Close()

	c, err := gps.NewDTLSSender(ip, gps.DefaultPort)
	if err != nil {
		log.Fatalf("failed to create gps client: %v", err)
	}
	defer c.Close()

	for {
		p, err := pos.Position(context.Background())
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
