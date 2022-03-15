package gps

//go:generate protoc rpc/gps.proto --go_out=. --go-grpc_out=. --go_opt=Mrpc/gps.proto=/rpc
