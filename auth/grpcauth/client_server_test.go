// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpcauth

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/auth"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	pb "google.golang.org/grpc/examples/features/proto/echo"
)

type user struct{}

func TestClientServerUnary(t *testing.T) {
	testSignKeys, err := auth.GenerateKeys()
	require.NoError(t, err)
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
			keyFile, err := os.CreateTemp("", "")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.Remove(keyFile.Name()) })

			certFile, err := os.CreateTemp("", "")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.Remove(certFile.Name()) })

			certPem, keyPem, err := auth.GenerateSelfSignedCert([]string{
				"x.test.example.com",
				"127.0.0.1",
				"localhost",
			}, pkix.Name{CommonName: "example"}, 1*time.Hour)
			require.NoError(t, err)

			_, err = keyFile.Write(keyPem)
			require.NoError(t, err)

			_, err = certFile.Write(certPem)
			require.NoError(t, err)

			cert, err := tls.LoadX509KeyPair(certFile.Name(), keyFile.Name())
			require.NoError(t, err)

			serverOpts := GRPCServerWithOauth2(test.keys, test.authorizer, credentials.NewServerTLSFromCert(&cert))

			s := grpc.NewServer(serverOpts...)
			srv := new(ecServer)
			pb.RegisterEchoServer(s, srv)

			defer s.Stop()

			lis, err := net.Listen("tcp", "127.0.0.1:")
			require.NoError(t, err)

			go func() {
				_ = s.Serve(lis)
			}()

			clientCreds, err := credentials.NewClientTLSFromFile(certFile.Name(), "localhost")
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
			conn, err := grpc.NewClient(lis.Addr().String(), clientOpts...)
			require.NoError(t, err)

			rgc := pb.NewEchoClient(conn)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := rgc.UnaryEcho(ctx, &pb.EchoRequest{Message: "1234"})
			if !test.expectForbidden {
				require.NoError(t, err)
				assert.Equal(t, "1234", resp.Message)
				require.NotNil(t, srv.ctx)

				claims, ok := auth.ClaimsFromContext[user](*srv.ctx)
				require.True(t, ok)

				expectedClaims := auth.UserClaims[user]{
					Email:  "it@unstable.build",
					UserID: "1234",
					Extra:  user{},
					Claims: jwt.Claims{
						Issuer:   "blue-auth",
						Subject:  "1234",
						Audience: []string{"blue-user"},
					},
				}
				claims.Expiry = nil
				claims.IssuedAt = nil
				claims.NotBefore = nil
				claims.ID = ""
				assert.Equal(t, expectedClaims, claims)
			} else {
				require.Error(t, err)
			}
			_ = conn.Close()
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
	ctx *context.Context
	pb.UnimplementedEchoServer
}

func (s *ecServer) UnaryEcho(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	s.ctx = &ctx
	return &pb.EchoResponse{Message: req.Message}, nil
}
