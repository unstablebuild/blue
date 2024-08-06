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

	"github.com/unstablebuild/blue/document"
	proto "github.com/unstablebuild/blue/document/rpc/proto"
	"github.com/unstablebuild/blue/encoding"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		if err == document.ErrPermissionDenied {
			err = status.Error(codes.PermissionDenied, "")
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
	if err == document.ErrPermissionDenied {
		err = status.Error(codes.PermissionDenied, "")
	}
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
		switch err {
		case document.ErrNotFound:
			return &proto.UpdateDocumentResponse{NotFound: true}, nil
		case document.ErrPreconditionFailed:
			return &proto.UpdateDocumentResponse{PreconditionFailed: true}, nil
		case document.ErrPermissionDenied:
			return nil, status.Error(codes.PermissionDenied, "")
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
		if err == document.ErrPermissionDenied {
			err = status.Error(codes.PermissionDenied, "")
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
	if err == document.ErrPermissionDenied {
		err = status.Error(codes.PermissionDenied, "")
	}
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
		if err == document.ErrPermissionDenied {
			err = status.Error(codes.PermissionDenied, "")
		}
		return err
	}

	return s.streamList(list, it)
}
