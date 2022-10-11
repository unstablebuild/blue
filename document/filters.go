package document

// WithFilter returns an updated slice of filters with a filter built with
// fieldPath value and op.
func WithFilter(
	in []Filter, fieldPath []string,
	value interface{}, op Op,
) []Filter {
	filter := Filter{
		Field: Field{
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
	in []Filter, projectID string,
) []Filter {
	return WithFilter(in, []string{"ProjectID"}, projectID, OpEqual)
}
