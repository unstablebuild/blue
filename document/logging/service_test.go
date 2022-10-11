package logging

import (
	"testing"

	"github.com/ernestrc/blue/document"
	test "github.com/ernestrc/blue/document/test"
)

func TestLoggingWithLogging(t *testing.T) {
	test.TestDocumentService(t, func(t *testing.T) document.Service {
		return WithLogging(document.NewInMemoryCache())
	})
}
