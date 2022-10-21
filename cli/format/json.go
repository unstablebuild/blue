package format

import (
	"encoding/json"
	"io"

	"github.com/ernestrc/blue/iterator"
)

// JSON returns an IteratorFormatter that formats elements into JSON objects.
func JSON[T any]() IteratorFormatter[T] {
	return jsonFormatter[T]{}
}

type jsonFormatter[T any] struct {
}

func (j jsonFormatter[T]) Format(w io.Writer, it iterator.Iterator[T]) error {
	e := json.NewEncoder(w)
	for {
		t, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		err = e.Encode(t)
		if err != nil {
			return err
		}
	}
}
