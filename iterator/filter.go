package iterator

// Filter wraps an iterator of type T and returns another iterator that will
// apply fn to each of the elements produced and filter out any elements for
// which fn returns false.
func Filter[T any](it Iterator[T], fn func(T) bool) Iterator[T] {
	return FromFunc(func() (ret T, ok bool, err error) {
		for {
			ret, ok = it.Next()
			if !ok {
				err = it.Err()
				return
			}
			ok = fn(ret)
			if !ok {
				continue
			}
			return
		}
	})
}
