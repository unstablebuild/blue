package grpc

import (
	"context"
	"crypto/tls"
	"net"
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

var (
	testSignKey = []byte("1234")
	denyAll     = auth.FuncAuthorizer(func(context.Context, auth.UserClaims, string) error {
		return auth.ErrForbidden
	})
)

type ecServer struct {
	pb.UnimplementedEchoServer
}

func (s *ecServer) UnaryEcho(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	return &pb.EchoResponse{Message: req.Message}, nil
}

func TestClientServer(t *testing.T) {
	suite := []struct {
		description     string
		setClientOpts   bool
		authorizer      auth.Authorizer[auth.UserClaims]
		expectForbidden bool
	}{
		{"rejects missing authorization header", false, auth.AuthorizeAll[auth.UserClaims](), true},
		{"accepts if valid token and authorizer grants", true, auth.AuthorizeAll[auth.UserClaims](), false},
		{"reject if valid token and authorizer does not grant", true, denyAll, true},
		{"reject if invalid token and authorizer does not grant", false, denyAll, true},
	}

	for _, test := range suite {
		test := test
		t.Run(test.description, func(t *testing.T) {
			cert, err := tls.LoadX509KeyPair(data.Path("x509/server_cert.pem"), data.Path("x509/server_key.pem"))
			require.NoError(t, err)

			serverOpts := GRPCServerWithOauth2(testSignKey, test.authorizer, credentials.NewServerTLSFromCert(&cert))

			s := grpc.NewServer(serverOpts...)
			pb.RegisterEchoServer(s, &ecServer{})

			defer s.Stop()

			lis, err := net.Listen("tcp", "127.0.0.1:")
			require.NoError(t, err)

			go s.Serve(lis)

			clientCreds, err := credentials.NewClientTLSFromFile(data.Path("x509/ca_cert.pem"), "x.test.example.com")
			require.NoError(t, err)

			idToken, err := auth.SignToken(testSignKey, "1234", "it@unstable.build", auth.RoleAdmin)
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
