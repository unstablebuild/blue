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

package docrpc

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	"github.com/unstablebuild/blue/document/docrpc/docpb"
	"github.com/unstablebuild/blue/document/doctest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestRPCDatastoreCustomServiceDesc(t *testing.T) {
	doctest.TestDocumentService(t, func(t *testing.T) document.Service {
		collectionName := "myCollection"
		marshaler := docbson.Marshaler()
		cache := document.NewInMemoryServiceWithMarshaler(marshaler)

		opts := []grpc.ServerOption{
			grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
				assert.Equal(t, 1, strings.Count(info.FullMethod, collectionName))
				return handler(ctx, req)
			}),
			grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
				assert.Equal(t, 1, strings.Count(info.FullMethod, collectionName))
				return handler(srv, ss)
			}),
		}

		addr, teardown := runDatastoreServerOverListener(t, cache,
			tcpListener, marshaler,
			func(reg grpc.ServiceRegistrar, srv docpb.DocumentStoreServer) {
				RegisterCollectionDocumentService(reg, srv, collectionName)
			}, opts...)

		opt := grpc.WithTransportCredentials(insecure.NewCredentials())
		cc, err := grpc.NewClient(addr.String(), opt)
		require.NoError(t, err)

		store := new(Client)
		store.InitWithCollection(cc, marshaler, collectionName)

		t.Cleanup(teardown)

		return store
	})
}
