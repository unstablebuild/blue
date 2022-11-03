package rpc

import "gopkg.in/mgo.v2/bson"

// MarshalerBSON returns a Marshaler backed by gopkg.in/mgo.v2/bson binary
// marshaler implementation.
func MarshalerBSON() Marshaler {
	return bsonMarshaler{}
}

type bsonMarshaler struct {
}

func (b bsonMarshaler) Marshal(doc interface{}) ([]byte, error) {
	return bson.Marshal(doc)
}
func (b bsonMarshaler) Unmarshal(data []byte, doc interface{}) error {
	return bson.Unmarshal(data, doc)
}
