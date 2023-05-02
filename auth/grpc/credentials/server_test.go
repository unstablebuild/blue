package credentials

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ernestrc/blue/auth/secretmanager"
	bluenet "github.com/ernestrc/blue/net"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCertSecretID = "myTLSCert"
	testKeySecretID  = "myTLSKey"
	goodCertPEM      = `-----BEGIN CERTIFICATE-----
MIIFtzCCA5+gAwIBAgIUBXNwlzVJM5WDS4Jmn1c92tQk4kswDQYJKoZIhvcNAQEL
BQAwajELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMRAwDgYDVQQHDAdPYWtsYW5k
MRYwFAYDVQQKDA1VbnN0YWJsZUJ1aWxkMQswCQYDVQQLDAJJVDEXMBUGA1UEAwwO
dW5zdGFibGUuYnVpbGQwIBcNMjMwNTAyMTgwNDA5WhgPMjEyMzA0MDgxODA0MDla
MGoxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTEQMA4GA1UEBwwHT2FrbGFuZDEW
MBQGA1UECgwNVW5zdGFibGVCdWlsZDELMAkGA1UECwwCSVQxFzAVBgNVBAMMDnVu
c3RhYmxlLmJ1aWxkMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAz6E2
3j6bmJvr9NfHcheLUBp6/SeM9NP295Y2sMI/7d0Yi2eoKRHbQuQvXkit3n6EHIms
/nmeBLHYM852fyBvOETTEw8GyUZNSwEYMBU1ndvcgTAFfpEJFlrfy2kK8ZzG+Dmm
sjGLoVhhoJ9TDGSfBmD/zxTlICZH49sRvPnnojHA/145Qm+Hm5wfe3riVXgasHl3
XRjZEVbTUFMf4LNRD3XMQCqa1UibC9IJokgKf0VGNsVXhngsh66vY2Nxy2V492gS
vnph6AYVvtMGcF+mjW6F8obcUJTgKW5PuhbXgN11hc4Or0EObGkJmvHPOCo+idlu
N7XrM8cD1hpn6nw1WbcGkDImcbsYFNbdmZCZFmE8OIZXzlGvSshXrPN4uzLDhVJE
l00tctW+ru543lY5YeIFKtkmumZKn7VxT6PoikxhGTS/GW3PSGm4xG8G7sQ6lxel
LJmEfQx0IhaJXMJo6BLITm8NcFNVbsJLLr7GrqOqU6k/49s03oaO+RqoSMVYCi0l
NrgZA1E8ByL7yh1CajW8uBzHEsaAhATe54JsH175R771GUjwOOVztpM2qfKY5ll0
l/OwS6ONufPNd8nIzLkTGtMRQ8vJZ/cV5nnPKfKt2KvSp4MerSRpw6yowuiBDywt
Ngzs1mfEtZGK8oHC8pup30bp/B3B8hEfwxN3qlECAwEAAaNTMFEwHQYDVR0OBBYE
FES+MZ3Rf+a4YatmCswOXHdXSOJlMB8GA1UdIwQYMBaAFES+MZ3Rf+a4YatmCswO
XHdXSOJlMA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggIBABRSiKhS
D8w/6ZIIa+l0IuGTamoCWCDvtpcApqbGyuR5C69/UHSaKuJLtATmd7twKrUQn0n0
YFjm4BVH6uz0vgMu8dCcGDjUcpCxz01Bx6jBBlBnR0q6I8pF1Xw8c31DYdTTbysy
js9LUphUN2qOSt8mOdDlH9fm+C1uXRLIr2qAs7r92gsknartyUlFjCe+Ne44z518
5cLfFZmlosUbNJrox3kXdfwa6AOam70zpn9iyR4HRpYj9a9+rNfqp3s8fX4iMYgd
kc8DF69TMQ9ANsLeqn/CNX5sC8hWjRtCcSmLrshuWRAv768LUMwqrA3TM6BWYPSX
mHonFFBcJcPEX4ZgqvdWbYLACsO66bQE0eiu6KfXzylS+sFDXhTL3p8Ftige2IuE
tjRJq61Ef3Beg+/D3LD7CZIieFlBMKCLH+YyxT8F2UcXEHiF8ogWJ+knhm8GhfJz
GNMLJA6C4SPCNRffHwLrbll3FCsMsYQrP80Ok2YJHHQDHgxgu+HVLRMrgjb0ig0r
eXsnsmuU6ATlxtjZ5OBcWWcQRd51UruAgpQOgt/3xeS5C939cAK1HJ0kaLfV8Lap
PPHkB1za7EenXdI5Vws0T+ruVuD6GTY6I6bjKkpcDeSHaxmcAJKJP8g8uyaHL18a
+JIE0JfOPD2cO+cvRS7/BnWWzTCsl2p/sZAS
-----END CERTIFICATE-----`
	goodKeyPEM = `-----BEGIN PRIVATE KEY-----
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
DY4P8fRD+Y2lkXroWp2wNzUk8tpj0U4=
-----END PRIVATE KEY-----`
)

var (
	goodKey  = secretmanager.SecretVersion{ID: testKeySecretID, Payload: []byte(goodKeyPEM)}
	goodCert = secretmanager.SecretVersion{ID: testCertSecretID, Payload: []byte(goodCertPEM)}
)

func TestServerTransport(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)

	t.Run("does not refresh until next refreshEvery", func(t *testing.T) {
		refreshEvery := 1 * time.Second
		svc := testSecretService{retCert: goodCert, retKey: goodKey}
		creds, err := newServerTransportCreds(&svc, testCertSecretID, testKeySecretID, refreshEvery)
		require.NoError(t, err)
		assert.Equal(t, int32(2), svc.called.Load())

		creds.Info()
		assert.Equal(t, int32(2), svc.called.Load())

		time.Sleep(refreshEvery)

		creds.Info()
		assert.Equal(t, int32(4), svc.called.Load())
	})

	t.Run("uses cached creds if service returns error", func(t *testing.T) {
		refreshEvery := 1 * time.Second
		svc := testSecretService{retCert: goodCert, retKey: goodKey}
		creds, err := newServerTransportCreds(&svc, testCertSecretID, testKeySecretID, refreshEvery)
		require.NoError(t, err)

		require.NoError(t, creds.OverrideServerName("test"))

		time.Sleep(refreshEvery)
		svc.retErr.Store(errors.New("boom"))

		require.NoError(t, creds.OverrideServerName("test"))
	})

	t.Run("returns error if first set of credentials could not be fetched", func(t *testing.T) {
		refreshEvery := 1 * time.Second
		svc := testSecretService{retCert: goodCert, retKey: goodKey}
		svc.retErr.Store(errors.New("boom"))

		_, err := newServerTransportCreds(&svc, testCertSecretID, testKeySecretID, refreshEvery)
		require.EqualError(t, err, "fetch transport credentials: access cert secret: boom")
	})

	t.Run("is goroutine-safe", func(t *testing.T) {
		const n = 500

		ctx := context.Background()
		svc := testSecretService{retCert: goodCert, retKey: goodKey}
		refreshEvery := 1 * time.Millisecond
		creds, err := newServerTransportCreds(&svc, testCertSecretID, testKeySecretID, refreshEvery)
		require.NoError(t, err)

		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				addr1, addr2 := &net.UDPAddr{Port: 1}, &net.UDPAddr{Port: 2}
				ch1, ch2 := make(chan bluenet.ReadResult), make(chan bluenet.ReadResult)
				c1 := bluenet.ChanConn(addr1, addr2, ch1, ch2)
				c2 := bluenet.ChanConn(addr2, addr1, ch2, ch1)
				go creds.ClientHandshake(ctx, "unstable.build", c1)
				go creds.ServerHandshake(c2)
				creds.Clone()
				creds.Info()
			}()
		}
		wg.Wait()
	})
}

type testSecretService struct {
	called  atomic.Int32
	retCert secretmanager.SecretVersion
	retKey  secretmanager.SecretVersion
	retErr  atomic.Value
}

func (s *testSecretService) AccessSecretLatest(
	ctx context.Context, ID string,
) (secretmanager.SecretVersion, error) {
	s.called.Add(1)
	if err := s.retErr.Load(); err != nil {
		return secretmanager.SecretVersion{}, err.(error)
	}
	if ID == testCertSecretID {
		return s.retCert, nil
	}
	if ID == testKeySecretID {
		return s.retKey, nil
	}
	return secretmanager.SecretVersion{}, errors.New("unknown key")
}
