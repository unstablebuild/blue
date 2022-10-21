package iterator

import (
	"github.com/ernestrc/blue/document"
)

// Iterator represents a dynamic collection of elements.
type Iterator[T any] interface {
	// Next returns the next element and true or nil and false
	// if there's no more elements in this Iterator.
	Next() (T, bool, error)
}

// Map maps an iterator of type T and returns another iterator that will
// apply fn to each of the elements produced.
func Map[T any, V any](it Iterator[T], fn func(T) V) Iterator[V] {
	return FromFunc[V](func() (ret V, ok bool, err error) {
		for {
			var t T
			t, ok, err = it.Next()
			if err != nil {
				return
			}
			if !ok {
				return
			}
			ret = fn(t)
			return
		}
	})
}

// FromSlice returns an iterator backed by the given slice of elements.
func FromSlice[T any](els []T) Iterator[T] {
	return &sliceIter[T]{els: els}
}

// FromFunc returns an Iterator backed by the provided function.
func FromFunc[T any](fn func() (T, bool, error)) Iterator[T] {
	return fnIter[T]{fn: fn}
}

// FromDocumentIterator maps a document.Iterator to
func FromDocumentIterator[T any](it document.Iterator) Iterator[T] {
	return FromFunc(func() (ret T, ok bool, err error) {
		if !it.HasNext() {
			return
		}
		ok = true
		err = it.NextTo(&ret)
		return
	})
}

type fnIter[T any] struct {
	fn func() (T, bool, error)
}

func (f fnIter[T]) Next() (T, bool, error) {
	return f.fn()
}

type sliceIter[T any] struct {
	els []T
}

func (i *sliceIter[T]) Next() (ret T, ok bool, err error) {
	if len(i.els) == 0 {
		return
	}
	ok = true
	ret = i.els[0]
	i.els = i.els[1:]
	return
}
