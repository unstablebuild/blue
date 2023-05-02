package rpc

import (
	"testing"

	"github.com/ernestrc/blue/document"
	proto "github.com/ernestrc/blue/document/rpc/proto"
	documenttest "github.com/ernestrc/blue/document/test"
	"github.com/ernestrc/blue/encoding/bson"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestRPCDatastoreCustomServiceDesc(t *testing.T) {
	documenttest.TestDocumentService(t, func(t *testing.T) document.Service {
		collectionName := "myCollection"
		marshaler := bson.Marshaler()
		cache := document.NewInMemoryServiceWithMarshaler(marshaler)
		addr, teardown := runDatastoreServerOverListener(t, cache,
			tcpListener, marshaler, func(reg grpc.ServiceRegistrar, srv proto.DocumentStoreServer) {
				RegisterCollectionDocumentService(reg, srv, collectionName)
			})

		cc, err := grpc.Dial(addr.String(), grpc.WithInsecure())
		require.NoError(t, err)

		store := new(Client)
		store.InitWithCollection(cc, marshaler, collectionName)

		t.Cleanup(teardown)

		return store
	})
}
