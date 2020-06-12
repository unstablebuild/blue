package cli

import "errors"

var (
	// ErrInvalidArgs is returned by CLI Run's method when invalid arguments
	// were passed.
	ErrInvalidArgs = errors.New("invalid usage of CLI")

	// ErrHelp returned when -h or --help flags are passed.
	ErrHelp = errors.New("help requested")
)
