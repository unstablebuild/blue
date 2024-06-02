package iterator

// Map maps an iterator of type T and returns another iterator that will
// apply fn to each of the elements produced.
func Map[T any, V any](it Iterator[T], fn func(T) V) Iterator[V] {
	return FromFunc(func() (ret V, ok bool, err error) {
		var t T
		t, ok = it.Next()
		if !ok {
			err = it.Err()
			return
		}
		ret = fn(t)
		return
	})
}
