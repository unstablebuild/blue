package document

import (
	"testing"

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

	testDatastore(t, func(t *testing.T) Service {
		collection := uuid.New().String()
		store, err := NewFireStore(testProjectID, collection, "")
		require.NoError(t, err)
		return store
	})
}
