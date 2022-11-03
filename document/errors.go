package document

import "errors"

// ErrNotFound is returned when the given document was not found in the store.
var ErrNotFound = errors.New("document not found")

// ErrAlreadyExists is returned when the given document already exists.
var ErrAlreadyExists = errors.New("document already exists")

// ErrPreconditionFailed is returned when a Precondition to Update
// is not met by the underlying document.
var ErrPreconditionFailed = errors.New("document pre-condition was not met")
