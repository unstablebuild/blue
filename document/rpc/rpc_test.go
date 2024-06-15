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
package rpc

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	proto "github.com/unstablebuild/blue/document/rpc/proto"
	documenttest "github.com/unstablebuild/blue/document/test"
	"github.com/unstablebuild/blue/encoding"
	"github.com/unstablebuild/blue/encoding/bson"
	"github.com/unstablebuild/blue/encoding/json"
	"github.com/unstablebuild/blue/encoding/toml"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func tcpListener() (net.Listener, error) {
	return net.Listen("tcp", ":0")
}

func runDatastoreServerOverListener(
	t *testing.T, other document.Service,
	listener func() (net.Listener, error),
	marshaler encoding.Marshaler,
	register func(grpc.ServiceRegistrar, proto.DocumentStoreServer),
	opts ...grpc.ServerOption,
) (net.Addr, func()) {
	gsrv := grpc.NewServer(opts...)

	srv := new(Server)
	register(gsrv, srv)
	srv.Init(other, marshaler)

	lis, err := listener()
	require.NoError(t, err)

	teardown := func() {
		gsrv.Stop()
		lis.Close()
	}

	go func() {
		_ = gsrv.Serve(lis)
	}()

	return lis.Addr(), teardown
}

func runDatastoreServer(
	t *testing.T, other document.Service, marshaler encoding.Marshaler,
) (net.Addr, func()) {
	return runDatastoreServerOverListener(t, other, tcpListener, marshaler,
		proto.RegisterDocumentStoreServer)
}

func testRPCDatastoreOverListener(
	t *testing.T, listener func() (net.Listener, error),
	marshaler encoding.Marshaler,
) {
	teardowns := []func(){}

	documenttest.TestDocumentService(t, func(t *testing.T) document.Service {
		cache := document.NewInMemoryServiceWithMarshaler(marshaler)
		addr, teardown := runDatastoreServerOverListener(t, cache,
			listener, marshaler, proto.RegisterDocumentStoreServer)
		teardowns = append(teardowns, teardown)

		store, err := NewClient(addr, marshaler,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
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
	tf, err := os.CreateTemp("", "plugin")
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
		testRPCDatastoreOverListener(t, tcpListener, bson.Marshaler())
	})

	t.Run("over Unix domain sockets", func(t *testing.T) {
		testRPCDatastoreOverListener(t, tempUnixListener, bson.Marshaler())
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

	t.Cleanup(func() {
		for _, fn := range teardowns {
			fn()
		}
	})

	for name, marshaler := range map[string]encoding.Marshaler{
		"bson": bson.Marshaler(),
		"toml": toml.Marshaler(),
		"json": json.Marshaler(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Run("writes by client/server are readable by underlying service", func(t *testing.T) {
				documenttest.TestDocumentService(t, func(t *testing.T) document.Service {
					cache := document.NewInMemoryServiceWithMarshaler(marshaler)
					addr, teardown := runDatastoreServer(t, cache, marshaler)
					teardowns = append(teardowns, teardown)

					store, err := NewClient(addr, marshaler,
						grpc.WithTransportCredentials(insecure.NewCredentials()))
					require.NoError(t, err)

					return interopHelper{read: cache, write: store}
				})
			})

			t.Run("writes by underlying service are readable by client/server", func(t *testing.T) {
				documenttest.TestDocumentService(t, func(t *testing.T) document.Service {
					cache := document.NewInMemoryServiceWithMarshaler(marshaler)
					addr, teardown := runDatastoreServer(t, cache, marshaler)
					teardowns = append(teardowns, teardown)

					store, err := NewClient(addr, marshaler,
						grpc.WithTransportCredentials(insecure.NewCredentials()))
					require.NoError(t, err)

					return interopHelper{read: store, write: cache}
				})
			})

			t.Run("single instance preconditions", func(t *testing.T) {
				documenttest.TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
					cache := document.NewInMemoryServiceWithMarshaler(marshaler)
					addr, teardown := runDatastoreServer(t, cache, marshaler)
					teardowns = append(teardowns, teardown)

					store, err := NewClient(addr, marshaler,
						grpc.WithTransportCredentials(insecure.NewCredentials()))
					require.NoError(t, err)

					return interopHelper{read: cache, write: store}
				})
			})
		})
	}
}
