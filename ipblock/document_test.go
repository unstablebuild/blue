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

package ipblock

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/firestore"
)

func newInMemoryDB(t *testing.T) document.Service {
	db := document.NewInMemoryService()
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestDocumentList(t *testing.T) {
	testList(t, func(t *testing.T, now func() time.Time) List {
		return newDocumentList(newInMemoryDB(t), now)
	})
}

// TestFirestoreList runs the contract against the Firestore
// emulator, which is what proves the document shape (IDs without a
// slash, a nullable timestamp) suits Firestore. It skips when the
// emulator is not installed.
func TestFirestoreList(t *testing.T) {
	teardown, err := firestore.RunFirestoreEmulator()
	if err != nil {
		t.Skipf("firestore emulator unavailable: %v", err)
	}
	t.Cleanup(func() { _ = teardown() })
	project := uuid.NewString()

	testList(t, func(t *testing.T, now func() time.Time) List {
		db, err := firestore.New(project, uuid.NewString(), "")
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		return newDocumentList(db, now)
	})
}

func TestDocumentList_StoredShape(t *testing.T) {
	ctx := context.Background()
	clock := newTestClock()
	db := newInMemoryDB(t)
	bl := newDocumentList(db, clock.Now)

	require.NoError(t, bl.Block(ctx, Block{Prefix: netip.MustParsePrefix("203.0.113.0/24")}))
	require.NoError(t, bl.Block(ctx, Block{
		Prefix: netip.MustParsePrefix("2001:db8::/64"), ExpiresAt: clock.Now().Add(time.Hour),
	}))

	var permanent map[string]any
	require.NoError(t, db.Get(ctx, "203.0.113.0_24", &permanent))
	assert.Equal(t, "203.0.113.0/24", field(t, permanent, "Prefix"))
	assert.Nil(t, field(t, permanent, "ExpiresAt"),
		"a block that never lapses must not carry a timestamp a TTL policy would act on")

	var expiring map[string]any
	require.NoError(t, db.Get(ctx, "2001:db8::_64", &expiring))
	assert.NotNil(t, field(t, expiring, "ExpiresAt"))
}

// field returns the named field of a raw document. Marshalers differ in
// the case of field names: Firestore keeps the Go names, BSON lowercases
// them.
func field(t *testing.T, doc map[string]any, name string) any {
	t.Helper()
	for k, v := range doc {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	require.Failf(t, "missing field", "%s not in %v", name, doc)
	return nil
}

func TestDocumentList_SkipsInvalidDocuments(t *testing.T) {
	ctx := context.Background()
	db := newInMemoryDB(t)
	bl := newDocumentList(db, newTestClock().Now)
	require.NoError(t, bl.Block(ctx, Block{Prefix: netip.MustParsePrefix("203.0.113.7/32")}))
	require.NoError(t, db.Set(ctx, "garbage", blockDocument{Prefix: "not a prefix"}))

	blocks, err := bl.List(ctx)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	_, ok, err := bl.Check(ctx, netip.MustParseAddr("203.0.113.7"))
	require.NoError(t, err)
	assert.True(t, ok, "one bad document must not lift the other blocks")
}

func TestNewDocumentList_PanicsWithoutDB(t *testing.T) {
	assert.Panics(t, func() { NewService(nil) })
}
