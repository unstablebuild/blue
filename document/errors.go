package document

import "errors"

var (
	// ErrNotFound is returned when the given document was not found in the store.
	ErrNotFound = errors.New("document not found")

	// ErrAlreadyExists is returned when the given document already exists.
	ErrAlreadyExists = errors.New("document already exists")

	// ErrPreconditionFailed is returned when a Precondition to Update
	// is not met by the underlying document.
	ErrPreconditionFailed = errors.New("document pre-condition was not met")

	// ErrPermissionDenied is returned when an operation is denied due to
	// permissions.
	ErrPermissionDenied = errors.New("access to document denied")
)
