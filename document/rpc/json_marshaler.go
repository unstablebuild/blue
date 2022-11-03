package rpc

import "encoding/json"

// MarshalerJSON returns a Marshaler backed by encoding/json text
// marshaler implementation.
func MarshalerJSON() Marshaler {
	return jsonMarshaler{}
}

type jsonMarshaler struct {
}

func (j jsonMarshaler) Marshal(doc interface{}) ([]byte, error) {
	return json.Marshal(doc)
}
func (j jsonMarshaler) Unmarshal(data []byte, doc interface{}) error {
	return json.Unmarshal(data, doc)
}
