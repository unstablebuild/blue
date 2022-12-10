package rpc

import (
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"runtime"
	"time"

	"github.com/ernestrc/blue/document"
	proto "github.com/ernestrc/blue/document/rpc/proto"
	"github.com/ernestrc/blue/encoding"
	"google.golang.org/grpc"
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
	opts = append(opts, grpc.WithDialer(
		func(_ string, _ time.Duration) (net.Conn, error) {
			conn, err := net.Dial(addr.Network(), addr.String())
			if err != nil {
				return nil, err
			}
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				// Make sure to set keep alive so that the connection doesn't die
				tcpConn.SetKeepAlive(true)
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
		return err
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
		return err
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
		return err
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
		return err
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
		return err
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
		return err
	}
	if errStr := next.GetError(); errStr != "" {
		return errors.New(errStr)
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
		return nil, err
	}
	return &rpcIterator{marshaler: c.marshaler, cc: res}, nil
}

func (c *Client) Close() error {
	if closer, ok := c.cc.(io.Closer); ok {
		return closer.Close()
	}
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
