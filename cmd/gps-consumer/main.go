package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/ernestrc/blue/gps"
	"github.com/ernestrc/blue/logging"
)

func main() {
	logging.SetDefaults(true)

	s, err := gps.NewServer("127.0.0.1", gps.DefaultPort)
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}

	defer s.Close()

	log.Fatal(s.ListenAndServe())
}
