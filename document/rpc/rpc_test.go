package rpc

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/ernestrc/blue/document"
	documenttest "github.com/ernestrc/blue/document/test"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/bson"
	"github.com/ernestrc/blue/encoding/json"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func tcpListener() (net.Listener, error) {
	return net.Listen("tcp", ":0")
}

func runDatastoreServerOverListener(
	t *testing.T, other document.Service, listener func() (net.Listener, error),
) (net.Addr, func()) {
	srv := NewServer(other, bson.Marshaler())
	lis, err := listener()
	require.NoError(t, err)

	teardown := func() {
		srv.Close()
		lis.Close()
	}

	go srv.Serve(lis)

	return lis.Addr(), teardown
}

func runDatastoreServer(t *testing.T, other document.Service) (net.Addr, func()) {
	return runDatastoreServerOverListener(t, other, tcpListener)
}

func testRPCDatastoreOverListener(t *testing.T, listener func() (net.Listener, error)) {
	teardowns := []func(){}

	documenttest.TestDocumentService(t, func(t *testing.T) document.Service {
		cache := document.NewInMemoryService()
		addr, teardown := runDatastoreServerOverListener(t, cache, listener)
		teardowns = append(teardowns, teardown)

		store, err := NewClient(addr, bson.Marshaler(), grpc.WithInsecure())
		require.NoError(t, err)

		return store
	})

	for _, fn := range teardowns {
		fn()
	}
}

// tempUnixListener creates a temp file and exposes it
// as a unix domain sockets net.Listener.
func tempUnixListener() (net.Listener, error) {
	tf, err := ioutil.TempFile("", "plugin")
	if err != nil {
		return nil, err
	}
	path := tf.Name()

	// Close the file and remove it because it has to not exist for
	// the domain socket.
	if err := tf.Close(); err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil {
		return nil, err
	}

	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	return l, nil
}

func TestRPC(t *testing.T) {
	t.Run("over TCP", func(t *testing.T) {
		testRPCDatastoreOverListener(t, tcpListener)
	})

	t.Run("over Unix domain sockets", func(t *testing.T) {
		testRPCDatastoreOverListener(t, tempUnixListener)
	})
}

type interopHelper struct {
	read  document.Service
	write document.Service
}

func (h interopHelper) Create(ctx context.Context, ID string, doc interface{}) error {
	return h.write.Create(ctx, ID, doc)
}

func (h interopHelper) Set(ctx context.Context, ID string, doc interface{}) error {
	return h.write.Set(ctx, ID, doc)
}

func (h interopHelper) Update(
	ctx context.Context, ID string, updates []document.Update,
	preconds ...document.Precondition,
) error {
	return h.write.Update(ctx, ID, updates, preconds...)
}

func (h interopHelper) Get(ctx context.Context, ID string, doc interface{}) error {
	return h.read.Get(ctx, ID, doc)
}

func (h interopHelper) Delete(ctx context.Context, ID string) error {
	return h.write.Delete(ctx, ID)
}

func (h interopHelper) List(ctx context.Context, filters []document.Filter) (document.Iterator, error) {
	return h.read.List(ctx, filters)
}

func (h interopHelper) Close() error {
	err1 := h.write.Close()
	err2 := h.read.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func TestRPCInterop(t *testing.T) {
	teardowns := []func(){}

	for name, marshaler := range map[string]encoding.Marshaler{
		"json": json.Marshaler(),
		"bson": bson.Marshaler(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Run("writes by client/server are readable by underlying service", func(t *testing.T) {
				documenttest.TestDocumentService(t, func(t *testing.T) document.Service {
					cache := document.NewInMemoryService()
					addr, teardown := runDatastoreServer(t, cache)
					teardowns = append(teardowns, teardown)

					store, err := NewClient(addr, marshaler, grpc.WithInsecure())
					require.NoError(t, err)

					return interopHelper{read: cache, write: store}
				})
			})

			t.Run("writes by underlying service are readable by client/server", func(t *testing.T) {
				documenttest.TestDocumentService(t, func(t *testing.T) document.Service {
					cache := document.NewInMemoryService()
					addr, teardown := runDatastoreServer(t, cache)
					teardowns = append(teardowns, teardown)

					store, err := NewClient(addr, marshaler, grpc.WithInsecure())
					require.NoError(t, err)

					return interopHelper{read: store, write: cache}
				})
			})
		})
	}

	for _, fn := range teardowns {
		fn()
	}
}
