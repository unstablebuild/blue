package rpc

import (
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"time"

	"github.com/ernestrc/blue/document"
	proto "github.com/ernestrc/blue/document/rpc/proto"
	"google.golang.org/grpc"
)

type Client struct {
	cc grpc.ClientConnInterface
	pb proto.DocumentStoreClient
}

// NewClient returns a grpc-based client that satisfies Service
// by relaying operations to remote datastore server. See NewServer
// for more details.
func NewClient(addr net.Addr, opts ...grpc.DialOption) (document.Service, error) {
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
	ret.Init(cc)
	return ret, nil
}

func (c *Client) Init(cc grpc.ClientConnInterface) {
	c.cc = cc
	c.pb = proto.NewDocumentStoreClient(cc)
}

func encodeCreateData(data interface{}) ([]byte, error) {
	if data == nil {
		panic("invalid nil data argument to Create/Set")
	}
	data, err := document.DerefCreateValue(reflect.ValueOf(data))
	if err != nil {
		return nil, err
	}

	return document.Encode(data, true), nil
}

func (c *Client) Create(
	ctx context.Context, ID string, data interface{},
) error {
	bytes, err := encodeCreateData(data)
	if err != nil {
		return err
	}

	req := proto.CreateDocumentRequest{Id: ID, Data: bytes}
	res, err := c.pb.Create(ctx, &req)
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
	bytes, err := encodeCreateData(data)
	if err != nil {
		return err
	}

	req := proto.SetDocumentRequest{Id: ID, Data: bytes}
	_, err = c.pb.Set(ctx, &req)
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
	u := makeProtoUpdates(updates)
	p := makeProtoPreconditions(preconds...)
	req := proto.UpdateDocumentRequest{Id: ID, Updates: u, Preconditions: p}
	res, err := c.pb.Update(ctx, &req)
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
	if err != nil {
		return err
	}
	if res.GetNotFound() {
		return document.ErrNotFound
	}

	data := res.GetData()
	err = document.SafeDecode(doc, data)
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
	if err != nil {
		return err
	}
	return nil
}

type rpcIterator struct {
	cc      proto.DocumentStore_ListClient
	next    *proto.ListDocumentResponse
	nextErr error
}

func (l *rpcIterator) HasNext() bool {
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

	return document.SafeDecode(doc, next.GetData())
}

func (l *rpcIterator) Close() error {
	return l.cc.CloseSend()
}

func makeModelFilter(
	slab map[string]interface{}, pf *proto.ListDocumentRequest_Filter,
) (document.Filter, error) {
	err := document.SafeDecode(&slab, pf.Data)
	if err != nil {
		return document.Filter{}, err
	}

	return document.Filter{
		Field: document.Field{
			FieldPath: pf.FieldPath,
			Value:     slab["."],
		},
		Op: document.Op(pf.Operation),
	}, nil
}

func makeModelFilters(filters []*proto.ListDocumentRequest_Filter) (
	ret []document.Filter, err error,
) {
	var slab map[string]interface{}
	for _, pf := range filters {
		var f document.Filter
		f, err = makeModelFilter(slab, pf)
		if err != nil {
			return
		}
		ret = append(ret, f)
	}
	return
}

func makeProtoFilter(
	slab map[string]interface{}, f document.Filter,
) proto.ListDocumentRequest_Filter {
	// this is just atrick to be able to re-use encode functionality
	slab["."] = f.Value

	return proto.ListDocumentRequest_Filter{
		FieldPath: f.FieldPath,
		Data:      document.Encode(slab, false),
		Operation: string(f.Op),
	}
}

func makeProtoFilters(filters []document.Filter) (
	ret []*proto.ListDocumentRequest_Filter, err error,
) {
	slab := make(map[string]interface{})
	for _, f := range filters {
		if len(f.Field.FieldPath) == 0 {
			panic("invalid List filter: empty zero-valued FieldPath")
		}
		pf := new(proto.ListDocumentRequest_Filter)
		*pf = makeProtoFilter(slab, f)

		ret = append(ret, pf)
	}
	return
}

func (c *Client) List(
	ctx context.Context, filters []document.Filter,
) (document.Iterator, error) {
	f, err := makeProtoFilters(filters)
	if err != nil {
		return nil, err
	}
	req := proto.ListDocumentRequest{Filters: f}
	res, err := c.pb.List(ctx, &req)
	if err != nil {
		return nil, err
	}
	return &rpcIterator{cc: res}, nil
}

func (c *Client) Close() error {
	if closer, ok := c.cc.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
