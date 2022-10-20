package rpc

import (
	"context"
	"errors"
	"net"

	"github.com/ernestrc/blue/document"
	proto "github.com/ernestrc/blue/document/rpc/proto"
	"google.golang.org/grpc"
)

// Server wraps another document.Service and exposes it through a grpc interface.
type Server struct {
	other document.Service
	srv   *grpc.Server
	proto.UnimplementedDocumentStoreServer
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(other document.Service, opt ...grpc.ServerOption) *Server {
	ret := new(Server)

	srv := grpc.NewServer()
	proto.RegisterDocumentStoreServer(srv, ret)

	ret.Init(other, srv)
	return ret
}

func (s *Server) Init(other document.Service, srv *grpc.Server) {
	s.srv = srv
	s.other = other
}

// Create satisfies proto.DocumentStoreServer
func (s *Server) Create(
	ctx context.Context, req *proto.CreateDocumentRequest,
) (res *proto.CreateDocumentResponse, err error) {
	id := req.GetId()
	data := req.GetData()

	var pr map[string]interface{}
	err = document.SafeDecode(&pr, data)
	if err != nil {
		return
	}

	err = s.other.Create(ctx, id, pr)
	if err != nil {
		if err == document.ErrAlreadyExists {
			err = nil
			res = &proto.CreateDocumentResponse{
				AlreadyExists: true,
			}
		}
		return
	}

	res = &proto.CreateDocumentResponse{}
	return
}

// Set satisfies proto.DocumentStoreServer
func (s *Server) Set(
	ctx context.Context, req *proto.SetDocumentRequest,
) (res *proto.DocumentResponse, err error) {
	id := req.GetId()
	data := req.GetData()

	var pr map[string]interface{}
	err = document.SafeDecode(&pr, data)
	if err != nil {
		return
	}

	err = s.other.Set(ctx, id, &pr)
	res = new(proto.DocumentResponse)
	return
}

// Update satisfies proto.DocumentStoreServer
func (s *Server) Update(
	ctx context.Context, req *proto.UpdateDocumentRequest,
) (res *proto.UpdateDocumentResponse, err error) {
	var updates []document.Update
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
		if err == document.ErrNotFound {
			res = &proto.UpdateDocumentResponse{NotFound: true}
			err = nil
		}
		return
	}
	res = new(proto.UpdateDocumentResponse)
	return
}

// Get satisfies proto.DocumentStoreServer
func (s *Server) Get(
	ctx context.Context, req *proto.GetDocumentRequest,
) (res *proto.GetDocumentResponse, err error) {
	id := req.GetId()

	var pr map[string]interface{}
	err = s.other.Get(ctx, id, &pr)
	if err != nil {
		if err == document.ErrNotFound {
			res = &proto.GetDocumentResponse{NotFound: true}
			err = nil
		}
		return
	}

	res = &proto.GetDocumentResponse{
		Data: document.Encode(pr, false),
	}
	return
}

// Delete satisfies proto.DocumentStoreServer
func (s *Server) Delete(
	ctx context.Context, req *proto.DeleteDocumentRequest,
) (res *proto.DocumentResponse, err error) {
	id := req.GetId()

	err = s.other.Delete(ctx, id)
	res = new(proto.DocumentResponse)
	return
}

func streamList(list proto.DocumentStore_ListServer, it document.Iterator) (err error) {
	for it.HasNext() {
		var pr map[string]interface{}
		err = it.NextTo(&pr)
		res := proto.ListDocumentResponse{}
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Data = document.Encode(pr, false)
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

// List satisfies proto.DocumentStoreServer
func (s *Server) List(
	req *proto.ListDocumentRequest, list proto.DocumentStore_ListServer,
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
