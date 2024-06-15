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
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"runtime"
	"strings"

	"github.com/unstablebuild/blue/document"
	proto "github.com/unstablebuild/blue/document/rpc/proto"
	"github.com/unstablebuild/blue/encoding"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	marshaler encoding.Marshaler
	cc        grpc.ClientConnInterface
	pb        proto.DocumentStoreClient
}

// NewClient returns a grpc-based client that satisfies Service
// by relaying operations to remote datastore server. See NewServer
// for more details.
func NewClient(addr net.Addr, m encoding.Marshaler, opts ...grpc.DialOption) (document.Service, error) {
	opts = append(opts, grpc.WithContextDialer(
		func(ctx context.Context, _ string) (net.Conn, error) {
			var d net.Dialer
			d.Deadline, _ = ctx.Deadline()
			conn, err := net.Dial(addr.Network(), addr.String())
			if err != nil {
				return nil, err
			}
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				// Make sure to set keep alive so that the connection doesn't die
				err := tcpConn.SetKeepAlive(true)
				if err != nil {
					return nil, fmt.Errorf("tcp conn set keep alive: %w", err)
				}
			}
			return conn, err
		},
	))
	cc, err := grpc.Dial("", opts...)
	if err != nil {
		return nil, err
	}

	ret := new(Client)
	runtime.SetFinalizer(ret, func(c *Client) { c.Close() })
	ret.Init(cc, m)
	return ret, nil
}

func (c *Client) Init(cc grpc.ClientConnInterface, m encoding.Marshaler) {
	c.cc = cc
	c.pb = proto.NewDocumentStoreClient(cc)
	c.marshaler = m
}

func (c *Client) Create(
	ctx context.Context, ID string, data interface{},
) error {
	bytes, err := c.encodeCreateData(data)
	if err != nil {
		return err
	}

	req := proto.CreateDocumentRequest{Id: ID, Data: bytes}
	res, err := c.pb.Create(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return convertRpcError(err)
	}
	if res.GetAlreadyExists() {
		return document.ErrAlreadyExists
	}
	return nil
}

func (c *Client) Set(
	ctx context.Context, ID string, data interface{},
) error {
	bytes, err := c.encodeCreateData(data)
	if err != nil {
		return err
	}

	req := proto.SetDocumentRequest{Id: ID, Data: bytes}
	_, err = c.pb.Set(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return convertRpcError(err)
	}
	return nil
}

func (c *Client) Update(
	ctx context.Context, ID string, updates []document.Update,
	preconds ...document.Precondition,
) error {
	if len(updates) == 0 {
		panic("invalid arguments: empty updates")
	}
	u := makeProtoUpdates(c.marshaler, updates)
	p := makeProtoPreconditions(c.marshaler, preconds...)
	req := proto.UpdateDocumentRequest{Id: ID, Updates: u, Preconditions: p}
	res, err := c.pb.Update(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return convertRpcError(err)
	}
	if res.GetNotFound() {
		return document.ErrNotFound
	}
	if res.GetPreconditionFailed() {
		return document.ErrPreconditionFailed
	}
	return nil
}

func (c *Client) Get(
	ctx context.Context, ID string, doc interface{},
) error {
	req := proto.GetDocumentRequest{Id: ID}
	res, err := c.pb.Get(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return convertRpcError(err)
	}
	if res.GetNotFound() {
		return document.ErrNotFound
	}

	data := res.GetData()
	err = document.SafeDecode(c.marshaler, doc, data)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Delete(
	ctx context.Context, ID string,
) error {
	req := proto.DeleteDocumentRequest{Id: ID}
	_, err := c.pb.Delete(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return convertRpcError(err)
	}
	return nil
}

type rpcIterator struct {
	marshaler encoding.Marshaler
	cc        proto.DocumentStore_ListClient
	next      *proto.ListDocumentResponse
	nextErr   error
}

func (l *rpcIterator) HasNext() bool {
	if l.next != nil {
		return true
	}

	m := new(proto.ListDocumentResponse)
	l.nextErr = l.cc.RecvMsg(m)
	if l.nextErr == io.EOF {
		return false
	}
	l.next = m
	return true
}

func (l *rpcIterator) NextTo(doc interface{}) error {
	if l.next == nil && l.nextErr == nil {
		if !l.HasNext() {
			return io.EOF
		}
	}
	next := l.next
	err := l.nextErr
	l.nextErr = nil
	l.next = nil
	if err != nil {
		return convertRpcError(err)
	}

	if errStr := next.GetError(); errStr != "" {
		switch {
		case strings.Contains(errStr, document.ErrNotFound.Error()):
			return document.ErrNotFound
		case strings.Contains(errStr, document.ErrAlreadyExists.Error()):
			return document.ErrAlreadyExists
		case strings.Contains(errStr, document.ErrPreconditionFailed.Error()):
			return document.ErrPreconditionFailed
		case strings.Contains(errStr, document.ErrPermissionDenied.Error()):
			return document.ErrPermissionDenied
		default:
			return errors.New(errStr)
		}
	}
	return document.SafeDecode(l.marshaler, doc, next.GetData())
}

func (l *rpcIterator) Close() error {
	return l.cc.CloseSend()
}

func (c *Client) List(
	ctx context.Context, filters []document.Filter,
) (document.Iterator, error) {
	f, err := makeProtoFilters(c.marshaler, filters)
	if err != nil {
		return nil, err
	}
	req := proto.ListDocumentRequest{Filters: f}
	res, err := c.pb.List(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, convertRpcError(err)
	}
	return &rpcIterator{marshaler: c.marshaler, cc: res}, nil
}

func (c *Client) Close() error {
	if closer, ok := c.cc.(io.Closer); ok {
		return closer.Close()
	}
	runtime.SetFinalizer(c, nil)
	return nil
}

func (c *Client) encodeCreateData(data interface{}) ([]byte, error) {
	if data == nil {
		panic("invalid nil data argument to Create/Set")
	}
	data, err := document.DerefCreateValue(reflect.ValueOf(data))
	if err != nil {
		return nil, err
	}

	return document.Encode(c.marshaler, data, true), nil
}

func convertRpcError(err error) error {
	status, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch status.Code() {
	case codes.FailedPrecondition:
		return document.ErrPreconditionFailed
	case codes.NotFound:
		return document.ErrNotFound
	case codes.AlreadyExists:
		return document.ErrAlreadyExists
	case codes.Unauthenticated, codes.PermissionDenied:
		return document.ErrPermissionDenied
	default:
		return err
	}
}
