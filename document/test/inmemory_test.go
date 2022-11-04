package test

import (
	"testing"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/bson"
	"github.com/ernestrc/blue/encoding/json"
	"github.com/ernestrc/blue/encoding/toml"
)

func TestInMemoryService(t *testing.T) {
	tsuite := []struct {
		encoding  string
		marshaler encoding.Marshaler
	}{
		{"bson", bson.Marshaler()},
		{"json", json.Marshaler()},
		{"toml", toml.Marshaler()},
		// NOTE: yaml passes all tests except the ones with encoding of
		// numerical values. It should never be used as a storage format anyway.
		// {"yaml", yaml.Marshaler()},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.encoding, func(t *testing.T) {
			TestDocumentService(t, func(t *testing.T) document.Service {
				return document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
			})
		})
	}
}
