// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package document

import (
	"context"
	"io"
)

const (
	// DefaultCreatedAtField is the document field that is automatically added
	// by Service implementors. See Service.Create for more info.
	DefaultCreatedAtField = "CreatedAt"

	// DefaultUpdatedAtField is the document field that is automatically updated
	// by Service implementors. See Service.Update for more info.
	DefaultUpdatedAtField = "UpdatedAt"

	// LowerCreatedAtField is the document field that is automatically added
	// by Service implementors when using a marshaler that defaults to lower case,
	// either directly or indirectly through another Service.
	LowerCreatedAtField = "createdat"

	// LowerUpdatedAtField is the document field that is automatically updated
	// by Service implementors when using a marshaler that defaults to lower case,
	// either directly or indirectly through another Service.
	LowerUpdatedAtField = "updatedat"
)

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

	// Set creates a document with the given data or updates it if it already exists.
	//
	// DefaultUpdatedAtField is automatically updated and clients can consume it
	// by adding the corresponding property in the document structure.
	//
	// See Create for more details.
	Set(ctx context.Context, ID string, doc interface{}) error

	// Update updates the document. The values at the given
	// field paths are replaced, but other fields of the stored document
	// are untouched.
	//
	// DefaultUpdatedAtField is automatically updated and clients can consume it
	// by adding the corresponding property in the document structure.
	Update(ctx context.Context, ID string,
		updates []Update, precond ...Precondition) error

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

// DroppableService wraps a Service and provides a method to delete all records
// efficiently.
type DroppableService interface {
	Service

	// Drop deletes all records in a document.Service. Implementors must guarantee
	// that (1) this is done efficiently and (2) the service remains functional
	// after this operation succeeds.
	Drop(context.Context) error
}

// Iterator is used to collect the the results obtained by List.
//
// HasNext is used to check how many results are left in the iterator.
// When HasNext returns false, a call to NextTo will panic.
//
// NextTo marshals the next document into the provided argument.
// It returns an error if marshaling fails. Once marshaled, the
// given document should not be re-used in the next call to NextTo
// otherwise map or slice fields could be overriden, depending on the
// implementation.
type Iterator interface {
	HasNext() bool
	NextTo(doc interface{}) error
	io.Closer
}

// Field represents a document field.
type Field struct {
	FieldPath []string
	Value     interface{}
}

// Update is used to indicate an update operation to a document field.
type Update Field

// Preconditions are optionally passed to Update to fail
// if the document state is not expected by the caller.
type Precondition Field

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
