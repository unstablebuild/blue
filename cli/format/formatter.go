package format

import (
	"io"

	"github.com/ernestrc/blue/iterator"
)

// IteratorFormatter defines the basic Iterator formatting method Format.
type IteratorFormatter interface {
	Format(io.Writer, iterator.Iterator) error
}
