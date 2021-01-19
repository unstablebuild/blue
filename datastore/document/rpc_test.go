package document

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func runDatastoreServer(t *testing.T, other Service) (string, func()) {
	srv := NewServer(other)
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	teardown := func() {
		srv.Close()
		lis.Close()
	}

	go srv.Serve(lis)

	return lis.Addr().String(), teardown
}

func TestRPC(t *testing.T) {
	teardowns := []func(){}
	testDatastore(t, func(t *testing.T) Service {
		cache := NewInMemoryCache()
		addr, teardown := runDatastoreServer(t, cache)
		teardowns = append(teardowns, teardown)

		store, err := NewClient(addr, grpc.WithInsecure())
		require.NoError(t, err)

		return store
	})

	for _, fn := range teardowns {
		fn()
	}
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
