package format

import (
	"io"

	"github.com/ernestrc/blue/iterator"
)

// IteratorFormatter defines the basic Iterator formatting method Format.
type IteratorFormatter[T any] interface {
	Format(io.Writer, iterator.Iterator[T]) error
}
