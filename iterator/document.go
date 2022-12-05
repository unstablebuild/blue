package iterator

import "github.com/ernestrc/blue/document"

// FromDocumentIterator maps a document.Iterator to
func FromDocumentIterator[T any](it document.Iterator) Iterator[T] {
	return FromFunc(func() (ret T, ok bool, err error) {
		if !it.HasNext() {
			err = it.Close()
			return
		}
		ok = true
		err = it.NextTo(&ret)
		return
	})
}
