package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	fileFlag          = flag.String("f", "", "Configuration file")
	debugFlag         = flag.Bool("d", false, "Run with verbose instrumentation.")
	fireStoreEmulator = flag.Bool("E", false, "Use FireStore Emulator as FireStore backend.")
)

func printUsageExit(code int) {
	flag.Usage()
	os.Exit(code)
}

func init() {
	flag.Parse()
	logging.SetDefaults(*debugFlag)
}

func newFireStore(cfg *appConfig) document.Service {
	store, err := document.NewFireStore(
		cfg.GC.ProjectID, cfg.GC.Collections.Source, cfg.GC.CredsFile)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize firestore")
	}

	_, err = store.List(context.Background(), nil)
	if err != nil {
		log.WithError(err).Fatal("failed to list firestore")
	}
	return store
}

func main() {
	cfg, err := sourceConfig(defaultReferenceConfig, *fileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		printUsageExit(1)
	}

	if *fireStoreEmulator {
		stopEmulator, err := document.RunFirestoreEmulator()
		if err != nil {
			log.WithError(err).Fatal("failed to start Firestore emulator")
		}
		defer stopEmulator()
	}

	store := newFireStore(cfg)
	defer store.Close()

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", cfg.GRPC.Port))
	if err != nil {
		log.WithError(err).WithFields(log.Fields{"port": cfg.GRPC.Port}).
			Fatal("failed to TCP listen")
	}
	opts := logging.WithServerLogger()
	if cfg.GRPC.TLS {
		creds, err := credentials.NewServerTLSFromFile(cfg.GRPC.CertFile, cfg.GRPC.KeyFile)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"key-file":  cfg.GRPC.KeyFile,
				"cert-file": cfg.GRPC.CertFile,
			}).Fatal("failed to TCP listen")
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	grpcServer := grpc.NewServer(opts...)
	// FIXME rpc.RegisterSourceServiceServer(grpcServer, service.NewSource(store))

	log.WithFields(log.Fields{
		"debug": *debugFlag,
		"port":  cfg.GRPC.Port,
	}).Info("starting Source server")

	log.Fatal(grpcServer.Serve(lis))
}
