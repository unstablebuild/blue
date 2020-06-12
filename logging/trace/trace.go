package trace

import "github.com/google/uuid"

// ID represents a trace ID propagated between services.
type ID string

// New returns a new trace.ID
func New() ID {
	return ID(uuid.New().String())
}
