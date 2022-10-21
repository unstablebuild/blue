package iterator

import (
	"github.com/ernestrc/blue/document"
)

// Iterator represents a dynamic collection of elements.
type Iterator interface {
	// Next returns the next element and true or nil and false
	// if there's no more elements in this Iterator.
	Next() (interface{}, bool, error)
}

// Slice returns an iterator backed by the given slice of elements.
func Slice(els []interface{}) Iterator {
	return &sliceIter{els: els}
}

// Func returns an Iterator backed by the provided function.
func Func(fn func() (interface{}, bool, error)) Iterator {
	return fnIter{fn: fn}
}

// FromDocumentIterator maps a document.Iterator to
func FromDocumentIterator[T interface{}](it document.Iterator) Iterator {
	return Func(func() (interface{}, bool, error) {
		if !it.HasNext() {
			return nil, false, nil
		}
		var temp T
		err := it.NextTo(&temp)
		if err != nil {
			return nil, false, err
		}
		return temp, false, nil
	})
}

type fnIter struct {
	fn func() (interface{}, bool, error)
}

func (f fnIter) Next() (interface{}, bool, error) {
	return f.fn()
}

type sliceIter struct {
	els []interface{}
}

func (i *sliceIter) Next() (interface{}, bool, error) {
	if len(i.els) == 0 {
		return nil, false, nil
	}
	el := i.els[0]
	i.els = i.els[1:]
	return el, true, nil
}
