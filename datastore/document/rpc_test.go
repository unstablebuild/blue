package document

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func tcpListener() (net.Listener, error) {
	return net.Listen("tcp", ":0")
}

func runDatastoreServerOverListener(
	t *testing.T, other Service, listener func() (net.Listener, error),
) (net.Addr, func()) {
	srv := NewServer(other)
	lis, err := listener()
	require.NoError(t, err)

	teardown := func() {
		srv.Close()
		lis.Close()
	}

	go srv.Serve(lis)

	return lis.Addr(), teardown
}

func runDatastoreServer(t *testing.T, other Service) (net.Addr, func()) {
	return runDatastoreServerOverListener(t, other, tcpListener)
}

func testRPCDatastoreOverListener(t *testing.T, listener func() (net.Listener, error)) {
	teardowns := []func(){}

	testDatastore(t, func(t *testing.T) Service {
		cache := NewInMemoryCache()
		addr, teardown := runDatastoreServerOverListener(t, cache, listener)
		teardowns = append(teardowns, teardown)

		store, err := NewClient(addr, grpc.WithInsecure())
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
	read  Service
	write Service
}

func (h interopHelper) Create(ctx context.Context, ID string, doc interface{}) error {
	return h.write.Create(ctx, ID, doc)
}

func (h interopHelper) Set(ctx context.Context, ID string, doc interface{}) error {
	return h.write.Set(ctx, ID, doc)
}

func (h interopHelper) Update(ctx context.Context, ID string, updates []Update) error {
	return h.write.Update(ctx, ID, updates)
}

func (h interopHelper) Get(ctx context.Context, ID string, doc interface{}) error {
	return h.read.Get(ctx, ID, doc)
}

func (h interopHelper) Delete(ctx context.Context, ID string) error {
	return h.write.Delete(ctx, ID)
}

func (h interopHelper) List(ctx context.Context, filters []Filter) (Iterator, error) {
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

	t.Run("writes by client/server are readable by underlying service", func(t *testing.T) {
		testDatastore(t, func(t *testing.T) Service {
			cache := NewInMemoryCache()
			addr, teardown := runDatastoreServer(t, cache)
			teardowns = append(teardowns, teardown)

			store, err := NewClient(addr, grpc.WithInsecure())
			require.NoError(t, err)

			return interopHelper{read: cache, write: store}
		})
	})

	t.Run("writes by underlying service are readable by client/server", func(t *testing.T) {
		testDatastore(t, func(t *testing.T) Service {
			cache := NewInMemoryCache()
			addr, teardown := runDatastoreServer(t, cache)
			teardowns = append(teardowns, teardown)

			store, err := NewClient(addr, grpc.WithInsecure())
			require.NoError(t, err)

			return interopHelper{read: store, write: cache}
		})
	})

	for _, fn := range teardowns {
		fn()
	}
}
