package grpc

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ernestrc/blue/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/examples/data"
	pb "google.golang.org/grpc/examples/features/proto/echo"
)

type user struct{}

func TestClientServerUnary(t *testing.T) {
	testSignKey := auth.SymmetricKey([]byte("1234"))
	testSignKeys := auth.StaticSymmetricKeys(testSignKey)
	denyAll := auth.FuncAuthorizer(func(context.Context, auth.UserClaims[user], string) error {
		return auth.ErrForbidden
	})
	priv := loadKey(t, "./testdata/jwk-priv.json")
	pub1 := loadKey(t, "./testdata/jwk-pub1.json.pub")
	pub2 := loadKey(t, "./testdata/jwk-pub2.json.pub")
	pub3 := loadKey(t, "./testdata/jwk-pub3.json.pub")
	multiKeys := auth.StaticAsymmetricKeys(priv, pub1, pub2, pub3)

	suite := []struct {
		description     string
		setClientOpts   bool
		authorizer      auth.Authorizer[user]
		expectForbidden bool
		keys            auth.Keys
	}{
		{"rejects missing authorization header", false, auth.AuthorizeAll[user](), true, testSignKeys},
		{"reject if valid token and authorizer does not grant", true, denyAll, true, testSignKeys},
		{"reject if invalid token and authorizer does not grant", false, denyAll, true, testSignKeys},
		{"accepts if exactly one of the keys verifies token and authorizer grants", true, auth.AuthorizeAll[user](), false, testSignKeys},
		{"accepts if at least one of the keys verifies token and authorizer grants", true, auth.AuthorizeAll[user](), false, multiKeys},
	}

	for _, test := range suite {
		test := test
		t.Run(test.description, func(t *testing.T) {
			cert, err := tls.LoadX509KeyPair(data.Path("x509/server_cert.pem"), data.Path("x509/server_key.pem"))
			require.NoError(t, err)

			serverOpts := GRPCServerWithOauth2(test.keys, test.authorizer, credentials.NewServerTLSFromCert(&cert))

			s := grpc.NewServer(serverOpts...)
			pb.RegisterEchoServer(s, &ecServer{})

			defer s.Stop()

			lis, err := net.Listen("tcp", "127.0.0.1:")
			require.NoError(t, err)

			go s.Serve(lis)

			clientCreds, err := credentials.NewClientTLSFromFile(data.Path("x509/ca_cert.pem"), "x.test.example.com")
			require.NoError(t, err)

			signKey, err := test.keys.Sign(context.Background())
			require.NoError(t, err)

			idToken, err := auth.SignToken(signKey, "1234", "it@unstable.build", user{}, 1*time.Hour)
			require.NoError(t, err)
			oauthToken := oauth2.Token{AccessToken: idToken}

			var clientOpts []grpc.DialOption
			if test.setClientOpts {
				clientOpts = GRPCClientWithOauth2(oauth2.StaticTokenSource(&oauthToken), clientCreds)
			} else {
				clientOpts = []grpc.DialOption{grpc.WithTransportCredentials(clientCreds)}
			}
			conn, err := grpc.Dial(lis.Addr().String(), clientOpts...)
			require.NoError(t, err)

			defer conn.Close()
			rgc := pb.NewEchoClient(conn)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := rgc.UnaryEcho(ctx, &pb.EchoRequest{Message: "1234"})
			if !test.expectForbidden {
				require.NoError(t, err)
				assert.Equal(t, "1234", resp.Message)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestClientServerStream(t *testing.T) {
	// stream re-uses all of unary's functionality so the roi of adding tests is rather low
}

func loadKey(t *testing.T, filename string) auth.Key {
	data, err := os.ReadFile(filename)
	require.NoError(t, err)

	var key auth.Key
	if strings.HasSuffix(filename, ".pub") {
		key, err = auth.LoadPublicKey(data)
	} else {
		key, err = auth.LoadPrivateKey(data)
	}
	require.NoError(t, err)
	return key
}

type ecServer struct {
	pb.UnimplementedEchoServer
}

func (s *ecServer) UnaryEcho(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	return &pb.EchoResponse{Message: req.Message}, nil
}
