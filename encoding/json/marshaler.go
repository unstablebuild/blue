package json

import (
	"encoding/json"

	"github.com/ernestrc/blue/encoding"
)

// Marshaler returns a JSON Marshaler.
func Marshaler() encoding.Marshaler {
	return jsonMarshaler{}
}

type jsonMarshaler struct {
}

func (j jsonMarshaler) Marshal(in interface{}) ([]byte, error) {
	return json.Marshal(in)
}

func (j jsonMarshaler) Unmarshal(data []byte, to interface{}) error {
	return json.Unmarshal(data, to)
}
