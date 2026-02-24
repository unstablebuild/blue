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

package test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/doclog"
	"github.com/unstablebuild/blue/document/docrpc"
	"github.com/unstablebuild/blue/document/docrpc/docpb"
	"github.com/unstablebuild/blue/document/doctest"
	"github.com/unstablebuild/blue/document/firestore"
	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestFirestoreIntegration(t *testing.T) {
	teardownEmulator := runFirestoreOrSkip(t)
	t.Cleanup(func() {
		teardownEmulator()
	})

	tsuite := []struct {
		encoding  string
		marshaler docmarshal.Marshaler
	}{
		{"toml", doctoml.Marshaler()},
		{"bson", docbson.Marshaler()},
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
			doctest.TestDocumentService(t, func(t *testing.T) document.Service {
				var svc document.Service

				collection := uuid.New().String()
				svc, err := firestore.New(testProjectID, collection, "")
				require.NoError(t, err)

				addr, teardown := runDatastoreServerOverListener(t, svc, tcpListener, marshaler)
				t.Cleanup(teardown)

				opts := grpc.WithTransportCredentials(insecure.NewCredentials())
				svc, err = docrpc.NewClient(addr, marshaler, opts)
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
		marshaler docmarshal.Marshaler
	}{
		{"toml", doctoml.Marshaler()},
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
				require.NoError(t, client.Create(ctx, "1",
					testStruct{Id: "1", Content: "a", Integer: 1}))

				var out testStruct
				require.NoError(t, firestore.Get(ctx, "1", &out))
				assert.Equal(t, "1", out.Id)
				assert.Equal(t, "a", out.Content)
				assert.NotZero(t, out.UpdatedAt)
			})

			t.Run("firestore create, client get", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, firestore.Create(ctx, "1",
					testStruct{Id: "1", Content: "a", Integer: 1}))

				var out testStruct
				require.NoError(t, client.Get(ctx, "1", &out))
				assert.Equal(t, "1", out.Id)
				assert.Equal(t, "a", out.Content)
				assert.NotZero(t, out.UpdatedAt)
			})

			t.Run("client create, firestore update, client get (string)", func(t *testing.T) {
				firestore, client := makeFirestoreClientPair(t, marshaler)
				require.NoError(t, client.Create(ctx, "1",
					testStruct{Id: "1", Content: "a", Integer: 1}))

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
				require.NoError(t, client.Create(ctx, "1",
					testStruct{Id: "1", Content: "a", Integer: 1}))

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
				require.NoError(t, client.Create(ctx, "1",
					testStruct{Id: "1", Content: "a", Integer: 1}))

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
				require.NoError(t, client.Create(ctx, "1",
					testStruct{Id: "1", Content: "a", Integer: 1}))

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
				require.NoError(t, firestore.Create(ctx, "1",
					testStruct{Id: "1", Content: "a", Integer: 1}))

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
				require.NoError(t, firestore.Create(ctx, "1",
					testStruct{Id: "1", Content: "a", Integer: 1}))

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
				require.NoError(t, client.Create(ctx, "1",
					testStruct{Id: "1", Content: "a", Integer: 1}))

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
				require.NoError(t, firestore.Create(ctx,
					"1", testStruct{Id: "1", Content: "a", Integer: 1}))

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
	marshaler docmarshal.Marshaler,
	opts ...grpc.ServerOption,
) (net.Addr, func()) {
	gsrv := grpc.NewServer(opts...)

	srv := new(docrpc.Server)
	srv.Init(other, marshaler)
	docpb.RegisterDocumentStoreServer(gsrv, srv)

	lis, err := listener()
	require.NoError(t, err)

	teardown := func() {
		gsrv.Stop()
		_ = lis.Close()
	}

	go func() {
		_ = gsrv.Serve(lis)
	}()

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

func makeFirestoreClientPair(t *testing.T, marshaler docmarshal.Marshaler) (
	fir, cli document.Service,
) {
	testProjectID := uuid.New().String()
	collection := uuid.New().String()
	firestore, err := firestore.New(testProjectID, collection, "")
	require.NoError(t, err)

	dbWithLogs := doclog.WithLogging(firestore, fmt.Sprintf("/Firestore/%s", collection))

	addr, teardown := runDatastoreServerOverListener(t, dbWithLogs, tcpListener, marshaler)
	t.Cleanup(teardown)

	client, err := docrpc.NewClient(addr, marshaler,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
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
