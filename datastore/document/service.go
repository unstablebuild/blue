package document

import (
	"context"
	"io"
)

// DefaultCreatedAtField is the document field that is automatically added
// by Service implementors. See Service.Create for more info.
const DefaultCreatedAtField = "CreatedAt"

// DefaultUpdatedAtField is the document field that is automatically updated
// by Service implementors. See Service.Update for more info.
const DefaultUpdatedAtField = "UpdatedAt"

// Service is the interface that encapsulates a document store service.
//
// The following rules must be followed in order to guarantee compatiblity
// across implementations:
//
//   - Document structures  must not contain embedded public fields.
//     This causes Update problems, as some implementations store the
//     embedded fields as a nested field, whereas others store them using
//     Go's internal representation.
//   - Document structures must not attempt to rename fields with tags. Some
//     implementations do not take tags, so Update operations, which use field
//     names string literals, would work for some implementations but not others.
type Service interface {
	// Create creates the document with the given data.
	// It returns an error if a document with the same ID already exists.
	// The data argument can be a map with string keys, a struct, or a pointer
	// to a struct. The map keys or exported struct fields become the
	// fields of the document.
	//
	// Pointers and the empty interface{} are permitted as
	// struct attributes or map values, and their elements processed recursively.
	//
	// DefaultCreatedAtField is automatically added and clients can consume it
	// by adding the corresponding property in the document structure.
	// Note that certain implementations might require special field tags.
	Create(ctx context.Context, ID string, doc interface{}) error

	// Update updates the document. The values at the given
	// field paths are replaced, but other fields of the stored document
	// are untouched.
	//
	// DefaultUpdatedAtField is automatically updated and clients can consume it
	// by adding the corresponding property in the document structure.
	Update(ctx context.Context, ID string, updates []Update) error

	// Get retrieves the document. If the document does not exist,
	// it returns a ErrNotFound error.
	// Parameter doc is used to populate the document's fields.
	// It can be a pointer to a map[string]interface{} or a pointer to a struct.
	Get(ctx context.Context, ID string, doc interface{}) error

	// Delete deletes the document. If the document doesn't exist,
	// it does nothing and returns no error.
	Delete(ctx context.Context, ID string) error

	// The List operation returns a page of all documents in the collection.
	// To return a subset of the collection, you can provide a set of filters.
	//
	// `cursor` represents the first item that this operation will evaluate.
	// If cursor is nil, then the start of the collection is assumed. Use
	// the value that was returned for nextCursor in the previous operation
	// to list the next set of documents. More data is available until nextCursor
	// is nil.
	//
	// The limit of documents returned is controled with the size of docs.
	// When nextCursor is nil, the returned n is used to determine how many
	// documents were marshaled into docs.
	List(ctx context.Context, filters []Filter) (Iterator, error)

	io.Closer
}

// Iterator is used to collect the the results obtained by List.
//
// HasNext is used to check how many results are left in the iterator.
// When HasNext returns false, a call to NextTo will panic.
//
// NextTo marshals the next document into the provided argument.
// It returns an error if marshaling fails.
type Iterator interface {
	HasNext() bool
	NextTo(doc interface{}) error
}

// Field represents a document field.
type Field struct {
	FieldPath []string
	Value     interface{}
}

// Update is used to indicate an update operation to a document field.
type Update Field

// Filter is used to construct a filter predicate in a List operation.
type Filter struct {
	Field
	Op
}

// Op represents a type of filter expression in a filter predicate.
type Op string

const (
	// OpEqual evalates true if the value in the Filter and the value
	// stored are equal.
	OpEqual Op = "=="
	// OpGreaterThan evalates true if the value in the Filter
	// is greater than the value stored in the collection.
	OpGreaterThan = ">"
	// OpGreaterThanEqual evalates true if the value in the Filter
	// is greater than or equal to the value stored in the collection.
	OpGreaterThanEqual = ">="
	// OpLessThan evalates true if the value in the Filter
	// is smaller than the value stored in the collection.
	OpLessThan = "<"
	// OpLessThanEqual evalates true if the value in the Filter
	// is smaller than or equal to the value stored in the collection.
	OpLessThanEqual = "<="
)
