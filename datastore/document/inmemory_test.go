package document

import (
	"testing"
)

func TestInMemoryCache(t *testing.T) {
	testDatastore(t, func(t *testing.T) Service {
		return NewInMemoryCache()
	})
}
