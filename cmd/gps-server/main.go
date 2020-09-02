package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pion/dtls/v2"
	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/gps"
	"github.com/ernestrc/blue/logging"
)

var (
	dtlsConfig = dtls.Config{
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_CCM_8},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	storeConfig = gps.DefaultMultiStoreConfig()

	debug       = flag.Bool("v", false, "Enable verbose logging")
	psk         = flag.String("k", "", "PSK key file name to use for DTLS handshake")
	pskIdentity = flag.String("i", "GPS DTLS Server", "PSK identity hint")
	dtlsPort    = flag.Int("P", gps.DefaultPort, "Listening DTLS port")
	httpPort    = flag.Int("X", 8080, "Listening HTTP port")
	host        = flag.String("h", "127.0.0.1", "Listening DTLS address")
	boltDbPath  = flag.String("b", ".gps-boltdb", "Path to mount the Bolt DB for GPS store")
	gcCredsFile = flag.String("c", "", "Google Cloud credentials file fore GPS store")
	gcProjectID = flag.String("p", "", "Google Cloud project ID fore GPS store")
)

func parseFlags() {
	flag.Parse()

	logging.SetDefaults(*debug)

	if *psk == "" {
		log.Fatal("Must pass -k flag")
	}

	if *gcProjectID == "" {
		log.Fatal("Must pass -p flag")
	}

	dtlsConfig.PSK = func(hint []byte) ([]byte, error) {
		log.Debugf("reading PSK file %s for hint %s", *psk, string(hint))
		return ioutil.ReadFile(*psk)
	}
	dtlsConfig.PSKIdentityHint = []byte(*pskIdentity)

	storeConfig.Bolt.DBPath = *boltDbPath
	storeConfig.Firestore.CredsFile = *gcCredsFile
	storeConfig.Firestore.ProjectID = *gcProjectID
}

func main() {
	parseFlags()

	store, err := gps.NewMultiStore(storeConfig)
	if err != nil {
		log.Fatalf("could not create store: %v", err)
	}

	s, err := gps.NewDTLSServer(*host, *dtlsPort, dtlsConfig, store)
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}
	defer s.Close()

	addr := fmt.Sprintf(":%d", *httpPort)
	httpHandler := logging.NewMiddleware(newAPI(store))

	go func() {
		log.Fatal(s.Serve())
	}()

	log.Infof("HTTP server listening at address %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpHandler))
}
