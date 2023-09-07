package test

import (
	"net"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/document/firestore"
	"github.com/ernestrc/blue/document/rpc"
	"github.com/ernestrc/blue/document/rpc/proto"
	"github.com/ernestrc/blue/document/test"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/bson"
	"github.com/ernestrc/blue/encoding/toml"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestFirestoreIntegration(t *testing.T) {
	teardownEmulator := runFirestoreOrSkip(t)
	t.Cleanup(func() {
		teardownEmulator()
	})

	tsuite := []struct {
		encoding  string
		marshaler encoding.Marshaler
	}{
		{"bson", bson.Marshaler()},
		{"toml", toml.Marshaler()},
		// FIXME dates are stored as string, which then are not interpreted correctly
		// as precondition or when json is used as a transport marshaler, in which case
		// the storage unmarshaler doesn't know how to decode.
		// {"json", json.Marshaler()},
		// NOTE: yaml passes all tests except the ones with encoding of
		// numerical values. It should never be used as a storage format anyway.
		// {"yaml", yaml.Marshaler()},
	}
	for _, tcase := range tsuite {
		marshaler := tcase.marshaler
		t.Run(tcase.encoding, func(t *testing.T) {
			testProjectID := uuid.New().String()
			test.TestDocumentService(t, func(t *testing.T) document.Service {
				var svc document.Service

				collection := uuid.New().String()
				svc, err := firestore.New(testProjectID, collection, "")
				require.NoError(t, err)

				addr, teardown := runDatastoreServerOverListener(t, svc, tcpListener, marshaler)
				t.Cleanup(teardown)

				svc, err = rpc.NewClient(addr, marshaler, grpc.WithInsecure())
				require.NoError(t, err)
				return svc
			})
		})
	}
}

func tcpListener() (net.Listener, error) {
	return net.Listen("tcp", ":0")
}

func runDatastoreServerOverListener(
	t *testing.T, other document.Service,
	listener func() (net.Listener, error),
	marshaler encoding.Marshaler,
	opts ...grpc.ServerOption,
) (net.Addr, func()) {
	gsrv := grpc.NewServer(opts...)

	srv := new(rpc.Server)
	proto.RegisterDocumentStoreServer(gsrv, srv)
	srv.Init(other, marshaler)

	lis, err := listener()
	require.NoError(t, err)

	teardown := func() {
		gsrv.Stop()
		lis.Close()
	}

	go gsrv.Serve(lis)

	return lis.Addr(), teardown
}

type testStruct struct {
	Id        string
	Content   string
	UpdatedAt time.Time
}

func runFirestoreOrSkip(t *testing.T) func() {
	teardown, err := firestore.RunFirestoreEmulator()
	if err != nil {
		t.Logf("problem with firestore emulator, skipping test: %s", err)
		t.SkipNow()
		return func() {}
	}
	return func() {
		err := teardown()
		if err != nil {
			t.Logf("error closing firestore emulator: %s", err)
		}
	}
}
