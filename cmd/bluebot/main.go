package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/logging"
)

const (
	serviceInitializeTimeout = 5 * time.Second
	slackBotTokenSecretID    = "bluebot-slack-oauth-token-prod"
	slackAppTokenSecretID    = "bluebot-slack-app-token-prod"
)

var (
	// compile-time variables
	Tag     = "development"
	Commit  = "HEAD"
	Version string

	gcCredsFile = flag.String("c", "",
		"Google Cloud credentials file for datastore")
	gcProjectID      = flag.String("p", "", "Google Cloud project ID")
	version          = flag.Bool("v", false, "Print version information to stdout")
	flagPprof        = flag.Bool("P", false, "Start pprof server at :8080")
	debug            = flag.Bool("V", false, "Enable verbose logging")
	jsonLogFormatter = flag.Bool("J", false, "Enable JSON log formatter for structured logs.")
	channelID        = flag.String("C", "", "Channel to post updates to")
	issuesCollection = flag.String("i", "blue-issues-beta", "Firestore collection for managing issues")
)

func init() {
	Version = fmt.Sprintf("%s (HEAD is %s)", Tag, Commit)
}

func main() {
	parseFlags()
	setupLogging()
	botToken, appToken := getSecrets()

	bot, err := newBot(*debug, *gcProjectID, *issuesCollection,
		*channelID, string(botToken), string(appToken))
	if err != nil {
		log.Fatalf("new bot: %v", err)
	}
	if err := bot.run(); err != nil {
		log.Fatalf("bot run: %v", err)
	}
}

func getSecrets() (botToken, appToken []byte) {
	secretStore, err := secretmanager.SecretStore(*gcProjectID, *gcCredsFile)
	if err != nil {
		log.Fatalf("secretmanager new secret store: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), serviceInitializeTimeout)
	defer cancel()

	botToken, err = secretStore.AccessSecret(ctx, slackBotTokenSecretID)
	if err != nil {
		log.Fatalf("access token secret: %v", err)
	}
	appToken, err = secretStore.AccessSecret(ctx, slackAppTokenSecretID)
	if err != nil {
		log.Fatalf("access app secret: %v", err)
	}
	return
}

func parseFlags() {
	flag.Parse()

	if *version {
		fmt.Printf("Bluebot (%s)\n", Version)
		os.Exit(0)
	}

	if *gcProjectID == "" {
		log.Fatal("Must pass -p flag")
	}

	if *channelID == "" {
		log.Fatal("Must pass -C flag")
	}

	if *flagPprof {
		go func() {
			if err := http.ListenAndServe(":8080", nil); err != http.ErrServerClosed {
				log.Errorf("listen and serve pprof: %v", err)
			}
		}()
	}
}

func setupLogging() {
	logging.SetDefaults(*debug)
	if *jsonLogFormatter {
		log.SetFormatter(logging.LogrusGCPFormatter{})
	}
}
