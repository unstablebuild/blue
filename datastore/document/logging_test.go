package document

import (
	"testing"
)

func TestLoggingWithLogging(t *testing.T) {
	testDatastore(t, func(t *testing.T) Service {
		return WithLogging(NewInMemoryCache())
	})
}
