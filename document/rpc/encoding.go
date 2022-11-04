package rpc

import (
	"errors"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding"
)

func safeDecode(m encoding.Marshaler, rcv interface{}, raw []byte) error {
	if !document.IsEncodeable(rcv) {
		return errors.New("receiver is not a pointer and not a map or is nil")
	}
	err := m.Unmarshal(raw, rcv)
	if err != nil {
		return err
	}
	return nil
}

func encode(m encoding.Marshaler, doc interface{}, addCreatedAt bool) []byte {
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
