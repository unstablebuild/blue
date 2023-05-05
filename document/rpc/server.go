package rpc

import (
	"context"
	"errors"

	"github.com/ernestrc/blue/document"
	proto "github.com/ernestrc/blue/document/rpc/proto"
	"github.com/ernestrc/blue/encoding"
)

// Server wraps another document.Service and exposes it through a grpc interface.
type Server struct {
	marshaler encoding.Marshaler
	other     document.Service
	proto.UnimplementedDocumentStoreServer
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(other document.Service, m encoding.Marshaler) *Server {
	ret := new(Server)
	ret.Init(other, m)
	return ret
}

func (s *Server) Init(other document.Service, m encoding.Marshaler) {
	s.other = other
	s.marshaler = m
}

// Create satisfies proto.DocumentStoreServer
func (s *Server) Create(
	ctx context.Context, req *proto.CreateDocumentRequest,
) (res *proto.CreateDocumentResponse, err error) {
	id := req.GetId()
	data := req.GetData()

	var pr map[string]interface{}
	err = document.SafeDecode(s.marshaler, &pr, data)
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
	err = document.SafeDecode(s.marshaler, &pr, data)
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
) (*proto.UpdateDocumentResponse, error) {
	updates, err := makeModelUpdates(s.marshaler, req.GetUpdates())
	if err != nil {
		return nil, err
	}
	preconds, err := makeModelPreconds(s.marshaler, req.GetPreconditions())
	if err != nil {
		return nil, err
	}
	// client should panic if no updates are passed
	// so the following is to avoid potential DOS from a malicious client.
	if len(updates) == 0 {
		err = errors.New("invalid request: no paths to update")
		return nil, err
	}
	err = s.other.Update(ctx, req.GetId(), updates, preconds...)
	if err != nil {
		if err == document.ErrNotFound {
			return &proto.UpdateDocumentResponse{NotFound: true}, nil
		}
		if err == document.ErrPreconditionFailed {
			return &proto.UpdateDocumentResponse{PreconditionFailed: true}, nil
		}
		return nil, err
	}
	return new(proto.UpdateDocumentResponse), nil
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
		Data: document.Encode(s.marshaler, pr, false),
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

func (s *Server) streamList(list proto.DocumentStore_ListServer, it document.Iterator) (err error) {
	for it.HasNext() {
		var pr map[string]interface{}
		err = it.NextTo(&pr)
		res := proto.ListDocumentResponse{}
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Data = document.Encode(s.marshaler, pr, false)
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
	filters, err := makeModelFilters(s.marshaler, req.GetFilters())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	it, err := s.other.List(ctx, filters)
	if err != nil {
		return err
	}

	return s.streamList(list, it)
}
