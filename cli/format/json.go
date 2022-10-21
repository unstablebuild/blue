package format

import (
	"encoding/json"
	"io"

	"github.com/ernestrc/blue/iterator"
)

// JSON returns an IteratorFormatter that formats elements into JSON objects.
func JSON() IteratorFormatter {
	return jsonFormatter{}
}

type jsonFormatter struct {
}

func (j jsonFormatter) Format(w io.Writer, it iterator.Iterator) error {
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
