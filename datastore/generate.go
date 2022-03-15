package datastore

//go:generate protoc rpc/datastore.proto --go_out=. --go-grpc_out=. --go_opt=Mrpc/datastore.proto=/rpc
