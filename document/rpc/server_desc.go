package rpc

import (
	"context"
	"fmt"
	"strings"

	proto "github.com/ernestrc/blue/document/rpc/proto"
	"github.com/ernestrc/blue/encoding"
	"google.golang.org/grpc"
)

// RegisterCollectionDocumentService registers the given srv with the given registrar
// with a custom service description name, that enables registering one service per
// collection. Client.InitWithCollection should be used on the client side to talk
// to a service registered via this function.
func RegisterCollectionDocumentService(
	registrar grpc.ServiceRegistrar, srv proto.DocumentStoreServer, collection string,
) {
	desc := proto.DocumentStore_ServiceDesc
	desc.ServiceName = fmt.Sprintf("proto.DocumentStore.%s", collection)
	for i, method := range desc.Methods {
		method := method
		desc.Methods[i].Handler = updateMethodInfoUnaryHandler(desc.ServiceName, method.Handler)
	}
	// stream method name is not mangling because it operates at a lower level
	// and the full method is defined when the path is matched, which is how
	// unary should work.
	registrar.RegisterService(&desc, srv)
}

func updateMethodInfoUnaryHandler(
	newServiceName string,
	// for some reason unary handlers are a private type!
	prev func(
		srv interface{}, ctx context.Context,
		dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor,
	) (interface{}, error),
) func(
	srv interface{}, ctx context.Context, dec func(interface{}) error,
	interceptor grpc.UnaryServerInterceptor) (
	interface{}, error,
) {
	return func(
		srv interface{}, ctx context.Context,
		dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor,
	) (interface{}, error) {
		if interceptor != nil {
			interceptor = updateMethodInfoUnaryInterceptor(newServiceName, interceptor)
		}
		return prev(srv, ctx, dec, interceptor)
	}
}

func updateMethodInfoUnaryInterceptor(
	newServiceName string, prev grpc.UnaryServerInterceptor,
) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		info.FullMethod = strings.ReplaceAll(info.FullMethod,
			"proto.DocumentStore", newServiceName)
		return prev(ctx, req, info, handler)
	}
}

// InitWithCollection initializes this Client with the given grpc connection, encoding marhshaler,
// and uses collection to suffix the service descriptor to enable multiple collection services
// registered in the same server. Server should be registered with grpc via RegisterCollectionDocumentService.
func (c *Client) InitWithCollection(cc grpc.ClientConnInterface, m encoding.Marshaler, collection string) {
	c.cc = cc
	c.pb = newDocumentStoreClient(cc, collection)
	c.marshaler = m
}

type documentStoreClient struct {
	cc         grpc.ClientConnInterface
	collection string
}

func newDocumentStoreClient(cc grpc.ClientConnInterface, collection string) proto.DocumentStoreClient {
	return &documentStoreClient{cc, collection}
}

func (c *documentStoreClient) Create(
	ctx context.Context, in *proto.CreateDocumentRequest, opts ...grpc.CallOption,
) (*proto.CreateDocumentResponse, error) {
	out := new(proto.CreateDocumentResponse)
	err := c.cc.Invoke(ctx,
		fmt.Sprintf("/proto.DocumentStore.%s/Create", c.collection), in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *documentStoreClient) Set(
	ctx context.Context, in *proto.SetDocumentRequest, opts ...grpc.CallOption,
) (*proto.DocumentResponse, error) {
	out := new(proto.DocumentResponse)
	err := c.cc.Invoke(ctx,
		fmt.Sprintf("/proto.DocumentStore.%s/Set", c.collection), in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *documentStoreClient) Update(
	ctx context.Context, in *proto.UpdateDocumentRequest, opts ...grpc.CallOption,
) (*proto.UpdateDocumentResponse, error) {
	out := new(proto.UpdateDocumentResponse)
	err := c.cc.Invoke(ctx,
		fmt.Sprintf("/proto.DocumentStore.%s/Update", c.collection), in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *documentStoreClient) Get(
	ctx context.Context, in *proto.GetDocumentRequest, opts ...grpc.CallOption,
) (*proto.GetDocumentResponse, error) {
	out := new(proto.GetDocumentResponse)
	err := c.cc.Invoke(ctx,
		fmt.Sprintf("/proto.DocumentStore.%s/Get", c.collection), in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *documentStoreClient) Delete(
	ctx context.Context, in *proto.DeleteDocumentRequest, opts ...grpc.CallOption,
) (*proto.DocumentResponse, error) {
	out := new(proto.DocumentResponse)
	err := c.cc.Invoke(ctx,
		fmt.Sprintf("/proto.DocumentStore.%s/Delete", c.collection), in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *documentStoreClient) List(
	ctx context.Context, in *proto.ListDocumentRequest, opts ...grpc.CallOption,
) (proto.DocumentStore_ListClient, error) {
	stream, err := c.cc.NewStream(ctx, &proto.DocumentStore_ServiceDesc.Streams[0],
		fmt.Sprintf("/proto.DocumentStore.%s/List", c.collection), opts...)
	if err != nil {
		return nil, err
	}
	x := &documentStoreListClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type documentStoreListClient struct {
	grpc.ClientStream
}

func (x *documentStoreListClient) Recv() (*proto.ListDocumentResponse, error) {
	m := new(proto.ListDocumentResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}
