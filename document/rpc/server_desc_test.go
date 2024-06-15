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
