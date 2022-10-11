package rpc

//go:generate protoc proto/datastore.proto --go_out=. --go-grpc_out=. --go_opt=Mproto/datastore.proto=/proto
