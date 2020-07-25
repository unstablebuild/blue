package main

import (
	"flag"
	"io/ioutil"

	"github.com/pion/dtls/v2"
	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/gps"
	"github.com/ernestrc/blue/logging"
)

var (
	config = dtls.Config{
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	debug       = flag.Bool("v", false, "Enable verbose logging")
	psk         = flag.String("k", "", "PSK key file name to use for DTLS handshake")
	pskIdentity = flag.String("i", "GPS DTLS Server", "PSK identity hint")
	port        = flag.Int("p", gps.DefaultPort, "Listening port")
	host        = flag.String("h", "127.0.0.1", "Listening address")
)

func main() {
	flag.Parse()
	logging.SetDefaults(*debug)

	if *psk == "" {
		log.Fatal("Must pass -k flag")
	}

	config.PSK = func(hint []byte) ([]byte, error) {
		log.Debugf("reading PSK file %s for hint %s", *psk, string(hint))
		return ioutil.ReadFile(*psk)
	}
	config.PSKIdentityHint = []byte(*pskIdentity)

	s, err := gps.NewDTLSServer(*host, *port, config,
		gps.LoggingReceiver("StaticTest"))
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}

	defer s.Close()

	log.Fatal(s.Serve())
}
