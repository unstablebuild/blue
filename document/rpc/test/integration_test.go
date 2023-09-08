package test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/document/firestore"
	doclog "github.com/ernestrc/blue/document/logging"
	"github.com/ernestrc/blue/document/rpc"
	"github.com/ernestrc/blue/document/rpc/proto"
	"github.com/ernestrc/blue/document/test"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/bson"
	"github.com/ernestrc/blue/encoding/toml"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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
		{"toml", toml.Marshaler()},
		{"bson", bson.Marshaler()},
		// NOTE: dates are stored as string, which then are not interpreted correctly
		// as precondition or when json is used as a transport marshaler, in which case
		// the storage unmarshaler doesn't know how to decode.
		// {"json", json.Marshaler()},
		// NOTE: yaml passes all tests except the ones with encoding of
		// numerical values. It should never be used as a storage format anyway.
		// {"yaml", yaml.Marshaler()},
	}
	for _, tcase := range tsuite {
		marshaler := tcase.marshaler
		testProjectID := uuid.New().String()

		t.Run(tcase.encoding, func(t *testing.T) {
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

func TestFirestoreBinaryCompatibility(t *testing.T) {
	teardownEmulator := runFirestoreOrSkip(t)
	t.Cleanup(func() {
		teardownEmulator()
	})

	tsuite := []struct {
		encoding  string
		marshaler encoding.Marshaler
	}{
		{"toml", toml.Marshaler()},
		// NOTE: bson changes the case of struct keys when going through the rpc calls
		// so it's not binary compatible with firestore.
		// {"bson", bson.Marshaler()},

		// NOTE: dates are stored as string, which then are not interpreted correctly
		// by firestore's unmarshaler.
		// {"json", json.Marshaler()},

		// NOTE: yaml passes all tests except the ones with encoding of
		// numerical values. It should never be used as a storage format anyway.
		// {"yaml", yaml.Marshaler()},
	}
	for _, tcase := range tsuite {
		marshaler := tcase.marshaler
		ctx := context.Background()

		t.Run(tcase.encoding, func(t *testing.T) {
			t.Run("client create, firestore get", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, client.Create(ctx, "1", testStruct{Id: "1", Content: "a", Integer: 1}))

				var out testStruct
				require.NoError(t, firestore.Get(ctx, "1", &out))
				assert.Equal(t, "1", out.Id)
				assert.Equal(t, "a", out.Content)
				assert.NotZero(t, out.UpdatedAt)
			})

			t.Run("firestore create, client get", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, firestore.Create(ctx, "1", testStruct{Id: "1", Content: "a", Integer: 1}))

				var out testStruct
				require.NoError(t, client.Get(ctx, "1", &out))
				assert.Equal(t, "1", out.Id)
				assert.Equal(t, "a", out.Content)
				assert.NotZero(t, out.UpdatedAt)
			})

			t.Run("client create, firestore update, client get (string)", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, client.Create(ctx, "1", testStruct{Id: "1", Content: "a", Integer: 1}))

				updates := make([]document.Update, 1)
				updates[0].FieldPath = []string{"Content"}
				updates[0].Value = "b"
				require.NoError(t, firestore.Update(ctx, "1", updates))

				var out testStruct
				require.NoError(t, client.Get(ctx, "1", &out))
				assert.Equal(t, "1", out.Id)
				assert.Equal(t, "b", out.Content)
				assert.NotZero(t, out.UpdatedAt)
			})

			t.Run("client create, firestore update, firestore get (string)", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, client.Create(ctx, "1", testStruct{Id: "1", Content: "a", Integer: 1}))

				updates := make([]document.Update, 1)
				updates[0].FieldPath = []string{"Content"}
				updates[0].Value = "b"
				require.NoError(t, firestore.Update(ctx, "1", updates))

				var out testStruct
				require.NoError(t, firestore.Get(ctx, "1", &out))
				assert.Equal(t, "1", out.Id)
				assert.Equal(t, "b", out.Content)
				assert.NotZero(t, out.UpdatedAt)
			})
			t.Run("client create, firestore update, client get", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, client.Create(ctx, "1", testStruct{Id: "1", Content: "a", Integer: 1}))

				updates := make([]document.Update, 1)
				updates[0].FieldPath = []string{"Content"}
				updates[0].Value = "b"
				require.NoError(t, firestore.Update(ctx, "1", updates))

				var out testStruct
				require.NoError(t, client.Get(ctx, "1", &out))
				assert.Equal(t, "1", out.Id)
				assert.Equal(t, "b", out.Content)
				assert.NotZero(t, out.UpdatedAt)
			})

			t.Run("client create, firestore update, firestore get (integer)", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, client.Create(ctx, "1", testStruct{Id: "1", Content: "a", Integer: 1}))

				updates := make([]document.Update, 1)
				updates[0].FieldPath = []string{"Integer"}
				updates[0].Value = 0
				require.NoError(t, firestore.Update(ctx, "1", updates))

				var out testStruct
				require.NoError(t, firestore.Get(ctx, "1", &out))
				assert.Equal(t, "1", out.Id)
				assert.Equal(t, int32(0), out.Integer)
				assert.NotZero(t, out.UpdatedAt)
			})
			t.Run("firestore create, firestore update, client get (integer)", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, firestore.Create(ctx, "1", testStruct{Id: "1", Content: "a", Integer: 1}))

				updates := make([]document.Update, 1)
				updates[0].FieldPath = []string{"Integer"}
				updates[0].Value = 0
				require.NoError(t, firestore.Update(ctx, "1", updates))

				var out testStruct
				require.NoError(t, client.Get(ctx, "1", &out))
				assert.Equal(t, "1", out.Id)
				assert.Equal(t, int32(0), out.Integer)
				assert.NotZero(t, out.UpdatedAt)
			})

			t.Run("firestore create, firestore update, client get", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, firestore.Create(ctx, "1", testStruct{Id: "1", Content: "a", Integer: 1}))

				updates := make([]document.Update, 1)
				updates[0].FieldPath = []string{"Content"}
				updates[0].Value = "b"
				require.NoError(t, firestore.Update(ctx, "1", updates))

				var out testStruct
				require.NoError(t, client.Get(ctx, "1", &out))
				assert.Equal(t, "1", out.Id)
				assert.Equal(t, "b", out.Content)
				assert.NotZero(t, out.UpdatedAt)
			})

			t.Run("client create, firestore list", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, client.Create(ctx, "1", testStruct{Id: "1", Content: "a", Integer: 1}))

				filters := make([]document.Filter, 1)
				filters[0].FieldPath = []string{"Content"}
				filters[0].Value = "a"
				filters[0].Op = document.OpEqual
				it, err := firestore.List(ctx, filters)
				require.NoError(t, err)

				assertListResults(t, it, []testStruct{
					{Id: "1", Content: "a", Integer: 1},
				})
			})

			t.Run("firestore create, client list", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, firestore.Create(ctx, "1", testStruct{Id: "1", Content: "a", Integer: 1}))

				filters := make([]document.Filter, 1)
				filters[0].FieldPath = []string{"Content"}
				filters[0].Value = "a"
				filters[0].Op = document.OpEqual
				it, err := client.List(ctx, filters)
				require.NoError(t, err)

				assertListResults(t, it, []testStruct{
					{Id: "1", Content: "a", Integer: 1},
				})
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
	srv.Init(other, marshaler)
	proto.RegisterDocumentStoreServer(gsrv, srv)

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
	Integer   int32
	Int64     int64
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

func makeFirestoreClientPair(t *testing.T, marshaler encoding.Marshaler) (
	fir, cli document.Service,
) {
	testProjectID := uuid.New().String()
	collection := uuid.New().String()
	firestore, err := firestore.New(testProjectID, collection, "")
	require.NoError(t, err)

	dbWithLogs := doclog.WithLogging(firestore, fmt.Sprintf("/Firestore/%s", collection))

	addr, teardown := runDatastoreServerOverListener(t, dbWithLogs, tcpListener, marshaler)
	t.Cleanup(teardown)

	client, err := rpc.NewClient(addr, marshaler, grpc.WithInsecure())
	require.NoError(t, err)

	return firestore, client
}

func assertListResults(
	t *testing.T, it document.Iterator, expectedElements []testStruct,
) {
	var i int
	if len(expectedElements) > 0 {
		// HasNext should be idempotent
		require.True(t, it.HasNext())
	}

	var actualElements []testStruct
	for it.HasNext() {
		var s testStruct
		err := it.NextTo(&s)
		require.NoError(t, err)
		i++
		assert.NotZero(t, s.UpdatedAt)
		s.UpdatedAt = time.Time{}
		actualElements = append(actualElements, s)
	}
	assert.False(t, it.HasNext())

	assert.Equal(t, len(expectedElements), i)
	assert.NoError(t, it.Close())
	assert.ElementsMatch(t, expectedElements, actualElements)
}
