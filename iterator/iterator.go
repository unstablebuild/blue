package iterator

// Iterator provides a convenient interface for iterating over
// chunks of structured or unstructured data such as
// a file of newline-delimited lines of text or a set of datastore documents.
type Iterator[T any] interface {
	// Next returns the next element and true or nil and false
	// if there's no more elements in this Iterator. Err should be
	// checked for any errors incurred during the lifecycle of this Iterator.
	// Note that if an error is found, it's up to the implementation as to
	// whether to return false and stop iteration or aggregate errors
	// and return at the end.
	Next() (T, bool)
	// Err returns the first error or an aggreation of the errors
	// encountered by the Iterator.
	Err() error
}
