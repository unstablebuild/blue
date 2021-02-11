package document

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"time"

	"github.com/ernestrc/blue/rpc"
	"google.golang.org/grpc"
)

type Client struct {
	cc grpc.ClientConnInterface
	pb rpc.DocumentStoreClient
}

// NewClient returns a grpc-based client that satisfies Service
// by relaying operations to remote datastore server. See NewServer
// for more details.
func NewClient(addr net.Addr, opts ...grpc.DialOption) (Service, error) {
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
		return nil, fmt.Errorf("grpc.Dial error: %v", err)
	}

	ret := new(Client)
	ret.Init(cc)
	return ret, nil
}

func (c *Client) Init(cc grpc.ClientConnInterface) {
	c.cc = cc
	c.pb = rpc.NewDocumentStoreClient(cc)
}

func encodeCreateData(data interface{}) ([]byte, error) {
	if data == nil {
		panic("invalid nil data argument to Create/Set")
	}
	data, err := derefCreateValue(reflect.ValueOf(data))
	if err != nil {
		return nil, err
	}

	return encode(data, true), nil
}

func (c *Client) Create(
	ctx context.Context, ID string, data interface{},
) error {
	bytes, err := encodeCreateData(data)
	if err != nil {
		return err
	}

	req := rpc.CreateDocumentRequest{Id: ID, Data: bytes}
	res, err := c.pb.Create(ctx, &req)
	if err != nil {
		return fmt.Errorf("pb.Create: %v", err)
	}
	if res.GetAlreadyExists() {
		return ErrAlreadyExists
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

	req := rpc.SetDocumentRequest{Id: ID, Data: bytes}
	_, err = c.pb.Set(ctx, &req)
	if err != nil {
		return fmt.Errorf("pb.Set: %v", err)
	}
	return nil
}

func makeProtoUpdates(updates []Update) (
	ret []*rpc.UpdateDocumentRequest_Update,
) {
	slab := make(map[string]interface{})
	for _, u := range updates {
		// re-use make filter logic
		f := Filter{Field: Field{FieldPath: u.FieldPath, Value: u.Value}}
		pf := makeProtoFilter(slab, f)
		pu := &rpc.UpdateDocumentRequest_Update{
			FieldPath: pf.FieldPath,
			Data:      pf.Data,
		}
		ret = append(ret, pu)
	}
	return
}

func makeModelUpdates(updates []*rpc.UpdateDocumentRequest_Update) (
	ret []Update, err error,
) {
	var slab map[string]interface{}
	var f Filter

	for _, u := range updates {
		// re-use make filter logic
		pf := rpc.ListDocumentRequest_Filter{
			FieldPath: u.FieldPath,
			Data:      u.Data,
		}
		f, err = makeModelFilter(slab, &pf)
		if err != nil {
			return
		}
		ret = append(ret, Update{
			FieldPath: f.FieldPath,
			Value:     f.Value,
		})
	}
	return
}

func (c *Client) Update(
	ctx context.Context, ID string, updates []Update,
) error {
	if len(updates) == 0 {
		panic("invalid arguments: empty updates")
	}
	u := makeProtoUpdates(updates)
	req := rpc.UpdateDocumentRequest{Id: ID, Updates: u}
	res, err := c.pb.Update(ctx, &req)
	if err != nil {
		return fmt.Errorf("pb.Update: %v", err)
	}
	if res.GetNotFound() {
		return ErrNotFound
	}
	return nil
}

func (c *Client) Get(
	ctx context.Context, ID string, doc interface{},
) error {
	req := rpc.GetDocumentRequest{Id: ID}
	res, err := c.pb.Get(ctx, &req)
	if err != nil {
		return fmt.Errorf("pb.Get: %v", err)
	}
	if res.GetNotFound() {
		return ErrNotFound
	}

	data := res.GetData()
	err = safeDecode(doc, data)
	if err != nil {
		return fmt.Errorf("failed to decode data: %v", err)
	}
	return nil
}

func (c *Client) Delete(
	ctx context.Context, ID string,
) error {
	req := rpc.DeleteDocumentRequest{Id: ID}
	_, err := c.pb.Delete(ctx, &req)
	if err != nil {
		return fmt.Errorf("pb.Delete: %v", err)
	}
	return nil
}

type rpcIterator struct {
	cc      rpc.DocumentStore_ListClient
	next    *rpc.ListDocumentResponse
	nextErr error
}

func (l *rpcIterator) HasNext() bool {
	m := new(rpc.ListDocumentResponse)
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

	return safeDecode(doc, next.GetData())
}

func (l *rpcIterator) Close() error {
	return l.cc.CloseSend()
}

func makeModelFilter(
	slab map[string]interface{}, pf *rpc.ListDocumentRequest_Filter,
) (Filter, error) {
	err := safeDecode(&slab, pf.Data)
	if err != nil {
		return Filter{}, err
	}

	return Filter{
		Field: Field{
			FieldPath: pf.FieldPath,
			Value:     slab["."],
		},
		Op: Op(pf.Operation),
	}, nil
}

func makeModelFilters(filters []*rpc.ListDocumentRequest_Filter) (
	ret []Filter, err error,
) {
	var slab map[string]interface{}
	for _, pf := range filters {
		var f Filter
		f, err = makeModelFilter(slab, pf)
		if err != nil {
			return
		}
		ret = append(ret, f)
	}
	return
}

func makeProtoFilter(
	slab map[string]interface{}, f Filter,
) rpc.ListDocumentRequest_Filter {
	// this is just atrick to be able to re-use encode functionality
	slab["."] = f.Value

	return rpc.ListDocumentRequest_Filter{
		FieldPath: f.FieldPath,
		Data:      encode(slab, false),
		Operation: string(f.Op),
	}
}

func makeProtoFilters(filters []Filter) (
	ret []*rpc.ListDocumentRequest_Filter, err error,
) {
	slab := make(map[string]interface{})
	for _, f := range filters {
		if len(f.Field.FieldPath) == 0 {
			panic("invalid List filter: empty zero-valued FieldPath")
		}
		pf := new(rpc.ListDocumentRequest_Filter)
		*pf = makeProtoFilter(slab, f)

		ret = append(ret, pf)
	}
	return
}

func (c *Client) List(
	ctx context.Context, filters []Filter,
) (Iterator, error) {
	f, err := makeProtoFilters(filters)
	if err != nil {
		return nil, err
	}
	req := rpc.ListDocumentRequest{Filters: f}
	res, err := c.pb.List(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("pb.List: %v", err)
	}
	return &rpcIterator{cc: res}, nil
}

func (c *Client) Close() error {
	if closer, ok := c.cc.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Server wraps another document.Service and exposes it through a grpc interface.
type Server struct {
	other Service
	srv   *grpc.Server
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(other Service, opt ...grpc.ServerOption) *Server {
	ret := new(Server)

	srv := grpc.NewServer()
	rpc.RegisterDocumentStoreServer(srv, ret)

	ret.srv = srv
	ret.other = other
	return ret
}

// Create satisfies rpc.DocumentStoreServer
func (s *Server) Create(
	ctx context.Context, req *rpc.CreateDocumentRequest,
) (res *rpc.CreateDocumentResponse, err error) {
	id := req.GetId()
	data := req.GetData()

	var proto map[string]interface{}
	err = safeDecode(&proto, data)
	if err != nil {
		return
	}

	err = s.other.Create(ctx, id, proto)
	if err != nil {
		if err == ErrAlreadyExists {
			err = nil
			res = &rpc.CreateDocumentResponse{
				AlreadyExists: true,
			}
		}
		return
	}

	res = &rpc.CreateDocumentResponse{}
	return
}

// Set satisfies rpc.DocumentStoreServer
func (s *Server) Set(
	ctx context.Context, req *rpc.SetDocumentRequest,
) (res *rpc.DocumentResponse, err error) {
	id := req.GetId()
	data := req.GetData()

	var proto map[string]interface{}
	err = safeDecode(&proto, data)
	if err != nil {
		return
	}

	err = s.other.Set(ctx, id, &proto)
	res = new(rpc.DocumentResponse)
	return
}

// Update satisfies rpc.DocumentStoreServer
func (s *Server) Update(
	ctx context.Context, req *rpc.UpdateDocumentRequest,
) (res *rpc.UpdateDocumentResponse, err error) {
	var updates []Update
	updates, err = makeModelUpdates(req.GetUpdates())
	if err != nil {
		return
	}
	// client should panic if no updates are passed
	// so the following is to avoid potential DOS from a malicious client.
	if len(updates) == 0 {
		err = errors.New("invalid request: no paths to update")
		return
	}
	err = s.other.Update(ctx, req.GetId(), updates)
	if err != nil {
		if err == ErrNotFound {
			res = &rpc.UpdateDocumentResponse{NotFound: true}
			err = nil
		}
		return
	}
	res = new(rpc.UpdateDocumentResponse)
	return
}

// Get satisfies rpc.DocumentStoreServer
func (s *Server) Get(
	ctx context.Context, req *rpc.GetDocumentRequest,
) (res *rpc.GetDocumentResponse, err error) {
	id := req.GetId()

	var proto map[string]interface{}
	err = s.other.Get(ctx, id, &proto)
	if err != nil {
		if err == ErrNotFound {
			res = &rpc.GetDocumentResponse{NotFound: true}
			err = nil
		}
		return
	}

	res = &rpc.GetDocumentResponse{
		Data: encode(proto, false),
	}
	return
}

// Delete satisfies rpc.DocumentStoreServer
func (s *Server) Delete(
	ctx context.Context, req *rpc.DeleteDocumentRequest,
) (res *rpc.DocumentResponse, err error) {
	id := req.GetId()

	err = s.other.Delete(ctx, id)
	res = new(rpc.DocumentResponse)
	return
}

func streamList(list rpc.DocumentStore_ListServer, it Iterator) (err error) {
	var proto map[string]interface{}
	for it.HasNext() {
		err = it.NextTo(&proto)
		res := rpc.ListDocumentResponse{}
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Data = encode(proto, false)
		}
		err = list.SendMsg(&res)
		if err != nil {
			return
		}
		if res.Error != "" {
			return
		}
	}
	return
}

// List satisfies rpc.DocumentStoreServer
func (s *Server) List(
	req *rpc.ListDocumentRequest, list rpc.DocumentStore_ListServer,
) error {
	ctx := context.Background()
	filters, err := makeModelFilters(req.GetFilters())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	it, err := s.other.List(ctx, filters)
	if err != nil {
		return err
	}

	return streamList(list, it)
}

// Serve accepts incoming connections on the listener lis.
// Serve returns when lis.Accept fails with fatal errors. lis will be closed when
// this method returns.
// Serve will return a non-nil error unless Stop or GracefulStop is called.
func (s *Server) Serve(lis net.Listener) error {
	return s.srv.Serve(lis)
}

// Close shuts down underlying grpc.Server.
func (s *Server) Close() error {
	s.srv.Stop()
	return nil
}
