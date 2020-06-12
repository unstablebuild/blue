package datastore

import "errors"

var (
	// ErrInvalidProtoMessage is used by FromProto implementations
	// when called with the wrong concrete type of proto.Message.
	ErrInvalidProtoMessage = errors.New("invalid proto message type")
)
