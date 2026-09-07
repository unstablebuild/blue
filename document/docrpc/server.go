// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docrpc

import (
	"context"
	"errors"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docrpc/docpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server wraps another document.Service and exposes it through a grpc interface.
type Server struct {
	marshaler docmarshal.Marshaler
	other     document.Service
	docpb.UnimplementedDocumentStoreServer
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(other document.Service, m docmarshal.Marshaler) *Server {
	ret := new(Server)
	ret.Init(other, m)
	return ret
}

func (s *Server) Init(other document.Service, m docmarshal.Marshaler) {
	s.other = other
	s.marshaler = m
}

// Create satisfies proto.DocumentStoreServer
func (s *Server) Create(
	ctx context.Context, req *docpb.CreateDocumentRequest,
) (res *docpb.CreateDocumentResponse, err error) {
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
			res = &docpb.CreateDocumentResponse{
				AlreadyExists: true,
			}
		}
		if err == document.ErrPermissionDenied {
			err = status.Error(codes.PermissionDenied, "")
		}
		return
	}

	res = &docpb.CreateDocumentResponse{}
	return
}

// Set satisfies proto.DocumentStoreServer
func (s *Server) Set(
	ctx context.Context, req *docpb.SetDocumentRequest,
) (res *docpb.DocumentResponse, err error) {
	id := req.GetId()
	data := req.GetData()

	var pr map[string]interface{}
	err = document.SafeDecode(s.marshaler, &pr, data)
	if err != nil {
		return
	}

	err = s.other.Set(ctx, id, &pr)
	if err == document.ErrPermissionDenied {
		err = status.Error(codes.PermissionDenied, "")
	}
	res = new(docpb.DocumentResponse)
	return
}

// Update satisfies proto.DocumentStoreServer
func (s *Server) Update(
	ctx context.Context, req *docpb.UpdateDocumentRequest,
) (*docpb.UpdateDocumentResponse, error) {
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
		switch err {
		case document.ErrNotFound:
			return &docpb.UpdateDocumentResponse{NotFound: true}, nil
		case document.ErrPreconditionFailed:
			return &docpb.UpdateDocumentResponse{PreconditionFailed: true}, nil
		case document.ErrPermissionDenied:
			return nil, status.Error(codes.PermissionDenied, "")
		}
		return nil, err
	}
	return new(docpb.UpdateDocumentResponse), nil
}

// Get satisfies proto.DocumentStoreServer
func (s *Server) Get(
	ctx context.Context, req *docpb.GetDocumentRequest,
) (res *docpb.GetDocumentResponse, err error) {
	id := req.GetId()

	var pr map[string]interface{}
	err = s.other.Get(ctx, id, &pr)
	if err != nil {
		if err == document.ErrNotFound {
			res = &docpb.GetDocumentResponse{NotFound: true}
			err = nil
		}
		if err == document.ErrPermissionDenied {
			err = status.Error(codes.PermissionDenied, "")
		}
		return
	}

	res = &docpb.GetDocumentResponse{
		Data: document.Encode(s.marshaler, pr, false),
	}
	return
}

// Delete satisfies proto.DocumentStoreServer
func (s *Server) Delete(
	ctx context.Context, req *docpb.DeleteDocumentRequest,
) (res *docpb.DocumentResponse, err error) {
	id := req.GetId()

	err = s.other.Delete(ctx, id)
	if err == document.ErrPermissionDenied {
		err = status.Error(codes.PermissionDenied, "")
	}
	res = new(docpb.DocumentResponse)
	return
}

func (s *Server) streamList(list docpb.DocumentStore_ListServer, it document.Iterator) (err error) {
	for it.HasNext() {
		var pr map[string]interface{}
		err = it.NextTo(&pr)
		res := docpb.ListDocumentResponse{}
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
	req *docpb.ListDocumentRequest, list docpb.DocumentStore_ListServer,
) error {
	filters, err := makeModelFilters(s.marshaler, req.GetFilters())
	if err != nil {
		return err
	}

	ctx := list.Context()
	it, err := s.other.List(ctx, filters)
	if err != nil {
		if err == document.ErrPermissionDenied {
			err = status.Error(codes.PermissionDenied, "")
		}
		return err
	}

	return s.streamList(list, it)
}
