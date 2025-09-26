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
