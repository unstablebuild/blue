package logging

import (
	"testing"

	"github.com/unstablebuild/blue/document"
	test "github.com/unstablebuild/blue/document/test"
)

func TestLoggingWithLogging(t *testing.T) {
	test.TestDocumentService(t, func(t *testing.T) document.Service {
		return WithLogging(document.NewInMemoryService(), "test")
	})
}
