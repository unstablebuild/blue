package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/jacobsa/go-serial/serial"
	"github.com/pion/dtls/v2"
	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/blue/gps"
	"github.com/ernestrc/blue/logging"
)

var (
	staticCoordinates = gps.Coordinates{
		Latitude:  39.932794,
		Longitude: -120.17963,
		Altitude:  15,
	}

	debug       = flag.Bool("v", false, "Enable verbose logging")
	psk         = flag.String("k", "", "PSK key file name to use for DTLS handshake")
	pskIdentity = flag.String("i", "GPS DTLS Client", "PSK identity hint")
	host        = flag.String("h", "127.0.0.1", "GPS server's hostname")
	port        = flag.Int("P", gps.DefaultPort, "GPS Server's listening port")
	cadenceSec  = flag.Int("c", 5,
		"Cadence as to which to send GPS coordinates to server")
	static       = flag.Bool("s", false, fmt.Sprintf("Use test static positioner with coordinates: %+v", staticCoordinates))
	serialDevice = flag.String("d", "/dev/ttyS0", "Serial device to use for serial GPS positioner if -s flag is not passed.")
	deviceID     = flag.String("I", "UnknownDevice", "Device ID used to identify GPS data")
	boltDbPath   = flag.String("b", "/tmp/gps-buffer-boltdb", "Path to mount the Bolt DB for GPS store")

	dtlsConfig = dtls.Config{
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	serialConfig = serial.OpenOptions{
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}
)

func getPositioner() (p gps.Positioner, err error) {
	if *static {
		staticCoordinates.DeviceID = *deviceID
		p = gps.NewStaticPositioner(staticCoordinates)
	} else {
		serialConfig.PortName = *serialDevice
		log.Debugf("reading serial GPS data from %s", serialConfig.PortName)
		p, err = gps.NewSerialDevicePositioner(*deviceID, serialConfig)
	}
	if err == nil {
		p = gps.WithLoggingPositioner(p, *deviceID)
	}
	return p, err
}

func main() {
	flag.Parse()
	logging.SetDefaults(*debug)

	p, err := getPositioner()
	if err != nil {
		log.Fatalf("could not instantiate GPS positioner: %s", err)
	}
	defer p.Close()

	dtlsConfig.PSK = func(hint []byte) ([]byte, error) {
		log.Debugf("reading PSK file %s for hint %s", *psk, string(hint))
		return ioutil.ReadFile(*psk)
	}
	dtlsConfig.PSKIdentityHint = []byte(*pskIdentity)

	s, err := gps.NewDTLSSender(*host, *port, dtlsConfig)
	if err != nil {
		log.Fatalf("failed to create gps client: %v", err)
	}
	defer s.Close()

	db, err := document.NewBolt(*boltDbPath, "gps-producer-buffer")
	if err != nil {
		log.Fatalf("failed to create bolt db: %v", err)
	}

	s = gps.WithBufferFallback(s, db)
	gps.SendPositionAtCadence(p, s, time.Duration(*cadenceSec)*time.Second)
}
