package rpc

import (
	"errors"

	"github.com/ernestrc/blue/document"
)

// Marshaler abstracts a text or binary marshaler
// which can be used with NewSchemeService to decide the encoding
// of the storage document files.
type Marshaler interface {
	Marshal(in interface{}) ([]byte, error)
	Unmarshal(data []byte, to interface{}) error
}

func safeDecode(m Marshaler, rcv interface{}, raw []byte) error {
	if !document.IsEncodeable(rcv) {
		return errors.New("receiver is not a pointer and not a map or is nil")
	}
	err := m.Unmarshal(raw, rcv)
	if err != nil {
		panic(err)
	}
	return nil
}

func encode(m Marshaler, doc interface{}, addCreatedAt bool) []byte {
	if addCreatedAt {
		doc = document.UpdateCreatedAtField(doc)
	} else {
		doc = document.UpdateUpdatedAtField(doc)
	}
	data, err := m.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return data
}
