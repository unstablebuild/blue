package credentials

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"sync/atomic"
	"time"

	"github.com/ernestrc/blue/auth/secretmanager"
	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials"
)

// ServerTransport uses the given projectID and credsFile to create a
// secretmanager client and returns a set of transport credentials that
// periodically fetch the latest cert and key via the provided certSecretID,
// and keySecretID.
//
// The secrets stored in secretmanager must store the data as a PEM block.
func ServerTransport(
	projectID, credsFile, certSecretID, keySecretID string,
	refreshEvery time.Duration,
) (credentials.TransportCredentials, error) {
	svc, err := secretmanager.NewService(projectID, credsFile)
	if err != nil {
		return nil, err
	}
	ret, err := newServerTransportCreds(svc, certSecretID,
		keySecretID, refreshEvery)
	return ret, nil
}

func newServerTransportCreds(
	svc secretService, certSecretID, keySecretID string,
	refreshEvery time.Duration,
) (credentials.TransportCredentials, error) {
	ret := &serverTransportCreds{
		svc:          svc,
		certSecretID: certSecretID,
		keySecretID:  keySecretID,
		refreshEvery: refreshEvery,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ret.updateCredentials(ctx); err != nil {
		return nil, fmt.Errorf("fetch transport credentials: %v", err)
	}
	return ret, nil
}

type secretService interface {
	AccessSecretLatest(ctx context.Context, ID string) (secretmanager.SecretVersion, error)
}

type serverTransportCreds struct {
	svc          secretService
	certSecretID string
	keySecretID  string
	refreshEvery time.Duration

	lastRefreshed atomic.Int64
	creds         atomic.Value
}

func (c *serverTransportCreds) ClientHandshake(
	ctx context.Context, auth string, conn net.Conn,
) (net.Conn, credentials.AuthInfo, error) {
	c.refreshMaybe()
	return c.lastCreds().ClientHandshake(ctx, auth, conn)
}

func (c *serverTransportCreds) ServerHandshake(conn net.Conn) (
	net.Conn, credentials.AuthInfo, error,
) {
	c.refreshMaybe()
	return c.lastCreds().ServerHandshake(conn)
}

func (c *serverTransportCreds) Info() credentials.ProtocolInfo {
	c.refreshMaybe()
	return c.lastCreds().Info()
}

func (c *serverTransportCreds) Clone() credentials.TransportCredentials {
	ret := &serverTransportCreds{
		svc:          c.svc,
		certSecretID: c.certSecretID,
		keySecretID:  c.keySecretID,
	}
	ret.lastRefreshed.Store(c.lastRefreshed.Load())
	ret.creds.Store(c.lastCreds().Clone())
	return ret
}

func (c *serverTransportCreds) OverrideServerName(name string) error {
	c.refreshMaybe()
	return c.lastCreds().OverrideServerName(name)
}

func (c *serverTransportCreds) lastCreds() credentials.TransportCredentials {
	return c.creds.Load().(credentials.TransportCredentials)
}

func (c *serverTransportCreds) log(level log.Level, msg string, args ...interface{}) {
	log.WithFields(log.Fields{
		logging.KeyClass:    "serverTransportCreds",
		logging.KeyCallType: "periodicRefresh",
	}).Logf(level, msg, args...)
}

func (c *serverTransportCreds) refreshMaybe() {
	nowNanos := time.Now().UnixNano()
	lastRefreshed := c.lastRefreshed.Load()
	if lastRefreshed+int64(c.refreshEvery) < int64(nowNanos) {
		//c.log(log.TraceLevel, "no need to refresh: %d + %d > %d",
		//	lastRefreshed, int64(c.refreshEvery), int64(nowNanos))
		return
	}
	// set to max int64, so no other goroutine attempts to refresh
	// and compare and swap to make sure that we are still the  only
	// ones attempting a refresh
	if !c.lastRefreshed.CompareAndSwap(lastRefreshed, math.MaxInt64) {
		// c.log(log.TraceLevel, "another goroutine is already working on refreshing certs")
		return
	}

	// do it async so there's no impact on establishing a new connection
	go func() {
		// attempt a rollback in any case. It should only succeed if updateCredentials
		// fails to refresh the credentials.
		defer c.lastRefreshed.CompareAndSwap(math.MaxInt64, lastRefreshed)

		ctx, cancel := context.WithTimeout(context.Background(), c.refreshEvery)
		defer cancel()

		if err := c.updateCredentials(ctx); err != nil {
			c.log(log.ErrorLevel, err.Error())
		}
	}()
}

func (c *serverTransportCreds) updateCredentials(ctx context.Context) error {
	certSecret, err := c.svc.AccessSecretLatest(ctx, c.certSecretID)
	if err != nil {
		return fmt.Errorf("access cert secret: %v", err)
	}
	keySecret, err := c.svc.AccessSecretLatest(ctx, c.keySecretID)
	if err != nil {
		return fmt.Errorf("access key secret: %v", err)
	}

	cert, err := tls.X509KeyPair(certSecret.Payload, keySecret.Payload)
	if err != nil {
		return fmt.Errorf("create x509 cert from PEM key pair: %v", err)
	}

	c.creds.Store(credentials.NewServerTLSFromCert(&cert))
	now := time.Now().UnixNano()
	c.lastRefreshed.Store(now)
	// c.log(log.TraceLevel, "successfully refreshed certs: %d", now)
	return nil
}
