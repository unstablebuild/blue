package datastore

import (
	"github.com/golang/protobuf/proto"
)

// Entity is the interface that wraps the basic FromProto and ToProto methods.
//
// FromProto takes a proto.Message and marshals it into the Entity. It panics
// if proto.Message is of an unexpected concrete type.
//
// ToProto returns this entity as a proto.Message.
type Entity interface {
	FromProto(proto.Message)
	ToProto() proto.Message
}
