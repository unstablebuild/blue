package iterator

import (
	"github.com/ernestrc/blue/document"
	multierr "github.com/ernestrc/go-multierror"
)

// Iterator provides a convenient interface for iterating over
// chunks of structured or unstructured data such as
// a file of newline-delimited lines of text or a set of datastore documents.
type Iterator[T any] interface {
	// Next returns the next element and true or nil and false
	// if there's no more elements in this Iterator. Err should be
	// checked for any errors incurred during the lifecycle of this Iterator.
	// Note that if an error is found, it's up to the implementation as to
	// whether to return false and stop iteration or aggregate errors
	// and return at the end.
	Next() (T, bool)
	// Err returns the first error or an aggreation of the errors
	// encountered by the Iterator.
	Err() error
}

// Map maps an iterator of type T and returns another iterator that will
// apply fn to each of the elements produced.
func Map[T any, V any](it Iterator[T], fn func(T) V) Iterator[V] {
	return FromFunc(func() (ret V, ok bool, err error) {
		for {
			var t T
			t, ok = it.Next()
			if !ok {
				err = it.Err()
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
	return &fnIter[T]{fn: fn}
}

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

type fnIter[T any] struct {
	err error
	fn  func() (T, bool, error)
}

func (f *fnIter[T]) Next() (T, bool) {
	t, ok, err := f.fn()
	if err != nil {
		f.err = multierr.Append(f.err, err)
		return t, false
	}
	return t, ok
}

func (f *fnIter[T]) Err() error {
	return f.err
}

type sliceIter[T any] struct {
	els []T
}

func (i *sliceIter[T]) Next() (ret T, ok bool) {
	if len(i.els) == 0 {
		return
	}
	ok = true
	ret = i.els[0]
	i.els = i.els[1:]
	return
}

func (i *sliceIter[T]) Err() error {
	return nil
}
