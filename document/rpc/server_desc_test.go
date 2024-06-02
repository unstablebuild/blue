package rpc

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	proto "github.com/unstablebuild/blue/document/rpc/proto"
	documenttest "github.com/unstablebuild/blue/document/test"
	"github.com/unstablebuild/blue/encoding/bson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestRPCDatastoreCustomServiceDesc(t *testing.T) {
	documenttest.TestDocumentService(t, func(t *testing.T) document.Service {
		collectionName := "myCollection"
		marshaler := bson.Marshaler()
		cache := document.NewInMemoryServiceWithMarshaler(marshaler)

		opts := []grpc.ServerOption{
			grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
				assert.Equal(t, 1, strings.Count(info.FullMethod, collectionName))
				return handler(ctx, req)
			}),
			grpc.StreamInterceptor(func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
				assert.Equal(t, 1, strings.Count(info.FullMethod, collectionName))
				return handler(srv, ss)
			}),
		}

		addr, teardown := runDatastoreServerOverListener(t, cache,
			tcpListener, marshaler, func(reg grpc.ServiceRegistrar, srv proto.DocumentStoreServer) {
				RegisterCollectionDocumentService(reg, srv, collectionName)
			}, opts...)

		opt := grpc.WithTransportCredentials(insecure.NewCredentials())
		cc, err := grpc.Dial(addr.String(), opt)
		require.NoError(t, err)

		store := new(Client)
		store.InitWithCollection(cc, marshaler, collectionName)

		t.Cleanup(teardown)

		return store
	})
}
