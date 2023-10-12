package iterator

import multierr "github.com/ernestrc/go-multierror"

// ToSlice consumes the given iterator and returns a slice with
// all of the elements produced.
func ToSlice[T any](it Iterator[T]) ([]T, error) {
	ret := make([]T, 0)
	for {
		t, ok := it.Next()
		if !ok {
			if err := it.Err(); err != nil {
				return nil, err
			}
			return ret, nil
		}
		ret = append(ret, t)
	}
}

// FromSlice returns an iterator backed by the given slice of elements.
func FromSlice[T any](els []T) Iterator[T] {
	return &sliceIter[T]{els: els}
}

// FromFunc returns an Iterator backed by the provided function.
func FromFunc[T any](fn func() (T, bool, error)) Iterator[T] {
	return &fnIter[T]{fn: fn}
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

// IsEmpty consumes the first element in i and returns true if it is empty
// or false if not and returns a new iterator that should be used instead of i.
func IsEmpty[T any](i Iterator[T]) (Iterator[T], bool) {
	el, ok := i.Next()
	if !ok {
		return FromSlice[T](nil), true
	}

	return FromFunc(func() (T, bool, error) {
		if ok {
			ok = false
			return el, true, nil
		}
		iEl, iOk := i.Next()
		if !iOk {
			return iEl, false, i.Err()
		}
		return iEl, iOk, nil
	}), false
}
