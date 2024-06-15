// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package grpc

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/unstablebuild/blue/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/examples/data"
	pb "google.golang.org/grpc/examples/features/proto/echo"
	"gopkg.in/go-jose/go-jose.v2/jwt"
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
			srv := new(ecServer)
			pb.RegisterEchoServer(s, srv)

			defer s.Stop()

			lis, err := net.Listen("tcp", "127.0.0.1:")
			require.NoError(t, err)

			go func() {
				_ = s.Serve(lis)
			}()

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
