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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
	"github.com/unstablebuild/blue/document/docrpc/docpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestClientClose(t *testing.T) {
	t.Run("does not close connection if it does not own connection", func(t *testing.T) {
		c := new(Client)
		mock := conn{}
		c.Init(&mock, doctoml.Marshaler())
		require.NoError(t, c.Close())

		assert.False(t, mock.called)
	})
	t.Run("does not close connection if it does not own connection", func(t *testing.T) {
		mock := conn{}
		svc := document.NewInMemoryService()
		marshaler := doctoml.Marshaler()
		addr, teardown := runDatastoreServerOverListener(t, svc, tcpListener, marshaler,
			docpb.RegisterDocumentStoreServer)
		defer teardown()
		c, err := NewClient(addr, marshaler, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		c.(*Client).cc = &mock
		require.NoError(t, c.Close())

		assert.True(t, mock.called)
	})
}

type conn struct {
	called bool
}

func (c *conn) Invoke(
	ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption,
) error {
	panic("unimplemented")
}

func (c *conn) NewStream(
	ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	panic("unimplemented")
}

func (c *conn) Close() error {
	c.called = true
	return nil
}
