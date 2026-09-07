// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package firestore

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/doctest"
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

	doctest.TestDocumentService(t, func(t *testing.T) document.Service {
		collection := uuid.New().String()
		store, err := New(testProjectID, collection, "")
		require.NoError(t, err)
		return store
	})
}

type myOtherEntity struct {
	Value     int
	UpdatedAt time.Time `firestore:",serverTimestamp"`
	CreatedAt time.Time `firestore:",serverTimestamp"`
}

func TestFirestorePreconditions(t *testing.T) {
	teardown := runFirestoreOrSkip(t)
	defer teardown()

	t.Run("Update does not fails if precondition is met", func(t *testing.T) {
		myID := "update_precondition_updated_at"
		ctx := context.Background()
		entity := myOtherEntity{}
		collection := uuid.New().String()
		s, err := New(uuid.New().String(), collection, "")
		require.NoError(t, err)
		defer func() { _ = s.Close() }()

		err = s.Create(ctx, myID, entity)
		require.NoError(t, err)

		// get updated server timestamp
		require.NoError(t, s.Get(ctx, myID, &entity))

		err = s.Update(ctx, myID,
			[]document.Update{
				{FieldPath: []string{"Value"}, Value: 1234},
			},
			document.Precondition{
				FieldPath: []string{document.DefaultUpdatedAtField},
				Value:     entity.UpdatedAt,
			},
		)
		require.NoError(t, err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)
		assert.Equal(t, 1234, e1.Value)
	})
}
