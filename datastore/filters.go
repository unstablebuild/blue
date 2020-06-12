package datastore

import "github.com/ernestrc/blue/datastore/document"

// WithFilter returns an updated slice of filters with a filter built with
// fieldPath value and op.
func WithFilter(
	in []document.Filter, fieldPath []string,
	value interface{}, op document.Op,
) []document.Filter {
	filter := document.Filter{
		Field: document.Field{
			FieldPath: fieldPath,
			Value:     value,
		},
		Op: op,
	}
	return append(in, filter)
}

// WithFilterProjectID returns an updated slice of filters which now
// will include documents with projectID.
func WithFilterProjectID(
	in []document.Filter, projectID string,
) []document.Filter {
	return WithFilter(in, []string{"ProjectID"}, projectID, document.OpEqual)
}
