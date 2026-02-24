// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
