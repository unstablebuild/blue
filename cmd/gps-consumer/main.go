package main

import (
	"github.com/pion/dtls/v2"
	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/gps"
	"github.com/ernestrc/blue/logging"
)

var config = dtls.Config{
	PSK: func(hint []byte) ([]byte, error) {
		return []byte{0xAB, 0xC1, 0x23}, nil
	},
	PSKIdentityHint:      []byte("Pion DTLS Client"),
	CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
	ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
}

func main() {
	logging.SetDefaults(true)

	s, err := gps.NewDTLSServer("127.0.0.1", gps.DefaultPort, config,
		gps.LoggingReceiver("StaticTest"))
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}

	defer s.Close()

	log.Fatal(s.Serve())
}
