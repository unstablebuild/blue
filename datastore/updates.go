package datastore

import (
	"github.com/ernestrc/blue/datastore/document"
)

// WithUpdateID returns an updated slice of updates
// which updates a document field `id`.
func WithUpdateID(in []document.Update, id string) []document.Update {
	update := document.Update{FieldPath: []string{"ID"}, Value: id}
	return append(in, update)
}

// WithUpdateDescription returns an updated slice of updates
// which updates a document field `desc`.
func WithUpdateDescription(in []document.Update, desc string) []document.Update {
	return append(in, document.Update{FieldPath: []string{"Description"}, Value: desc})
}
