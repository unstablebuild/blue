package iterator

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
