package firestore

import (
	"testing"

	"github.com/unstablebuild/blue/document"
	documenttest "github.com/unstablebuild/blue/document/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func runFirestoreOrSkip(t *testing.T) func() {
	teardown, err := RunFirestoreEmulator()
	if err != nil {
		t.Logf("problem with firestore emulator, skipping test: %s", err)
		t.SkipNow()
		return func() {}
	}
	return func() {
		err := teardown()
		if err != nil {
			t.Logf("error closing firestore emulator: %s", err)
		}
	}
}

func TestFirestore(t *testing.T) {
	testProjectID := uuid.New().String()

	teardown := runFirestoreOrSkip(t)
	defer teardown()

	documenttest.TestDocumentService(t, func(t *testing.T) document.Service {
		collection := uuid.New().String()
		store, err := New(testProjectID, collection, "")
		require.NoError(t, err)
		return store
	})
}
