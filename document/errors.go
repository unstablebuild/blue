package document

import "errors"

// ErrNotFound is returned when the given document was not found in the store.
var ErrNotFound = errors.New("document not found")

// ErrAlreadyExists is returned when the given document already exists.
var ErrAlreadyExists = errors.New("document already exists")
