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
package test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/go-multierror"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/retry"
)

type FnServiceFactory func(t *testing.T) document.Service

type Reaper interface {
	reapChains() error
}

type Segador struct {
	Name          string
	Traits        map[string]interface{}
	internalField string
}

func Alice() Segador {
	return MakeSegador(
		withName("Alice"),
		withTrait("dob", "1989-11-03"),
		withTrait("years", float64(30)),
		withTrait("fancy", false),
	)
}

func Bob() Segador {
	return MakeSegador(
		withName("Bob"),
		withTrait("dob", "1989-11-03"),
		withTrait("years", float64(30)),
		withTrait("fancy", true),
		withTrait("sister", map[string]interface{}{
			"Name": "Alice",
			"Traits": map[string]interface{}{
				"dob":   "1989-11-03",
				"years": float64(30),
				"fancy": false,
			},
		}),
	)
}

func (s *Segador) reapChains() error {
	return nil
}

func (s *Segador) assertEqualMap(t *testing.T, svc document.Service, target map[string]interface{}) {
	// use service to encode so we're testing encoding/decoding from/to map.
	// otherwise we need to make assumptions about how the fields names are serialized
	// which greatly defer depending on impls.
	id := uuid.New().String()
	ctx := context.Background()
	res := make(map[string]interface{})
	err := svc.Create(ctx, id, s)
	require.NoError(t, err)
	err = svc.Get(ctx, id, &res)
	require.NoError(t, err)
	delete(res, document.DefaultCreatedAtField)
	delete(res, document.DefaultUpdatedAtField)
	delete(res, strings.ToLower(document.DefaultCreatedAtField))
	delete(res, strings.ToLower(document.DefaultUpdatedAtField))

	delete(target, document.DefaultCreatedAtField)
	delete(target, document.DefaultUpdatedAtField)
	delete(target, strings.ToLower(document.DefaultCreatedAtField))
	delete(target, strings.ToLower(document.DefaultUpdatedAtField))

	// this is a hack, which enables partitions tests to work seamlessly
	for k := range res {
		if strings.HasPrefix(k, "__") {
			delete(res, k)
		}
	}
	for k := range target {
		if strings.HasPrefix(k, "__") {
			delete(target, k)
		}
	}

	assert.EqualValues(t, res, target)
}

type segadorOpt func(*Segador) *Segador

func MakeSegador(opts ...segadorOpt) (s Segador) {
	for _, o := range opts {
		s = *o(&s)
	}
	return
}

func withName(name string) segadorOpt {
	return func(s *Segador) *Segador {
		s.Name = name
		return s
	}
}

func withTrait(k string, value interface{}) segadorOpt {
	return func(s *Segador) *Segador {
		if s.Traits == nil {
			s.Traits = make(map[string]interface{})
		}
		s.Traits[k] = value
		return s
	}
}

var (
	alice = Alice()

	bob = Bob()
)

type myOtherEntity struct {
	Value     int
	UpdatedAt time.Time `firestore:",serverTimestamp"`
	CreatedAt time.Time `firestore:",serverTimestamp"`
}

func testDatastoreCreate(t *testing.T, serviceFactory FnServiceFactory) {
	ctx := context.Background()

	t.Run("Create returns error if data is not a struct or a map", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		assert.Error(t, s.Create(ctx, "my1234", 1234))

		var myReaper Reaper //nolint:gosimple
		myReaper = &bob
		assert.Error(t, s.Create(ctx, "my1234", &myReaper))
	})

	t.Run("Create with data argument with several levels of indirection", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		bobRef := &bob

		err := s.Create(ctx, "bobID", &bobRef)
		require.NoError(t, err)

		var myVal Segador
		err = s.Get(ctx, "bobID", &myVal)
		require.NoError(t, err)
		assert.EqualValues(t, bob, myVal)
	})

	t.Run("Create returns ErrAlreadyExists if attempt to create a document that already exists", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		err := s.Create(ctx, "aliceID", bob)
		require.NoError(t, err)

		assert.Equal(t, document.ErrAlreadyExists, s.Create(ctx, "aliceID", bob))
	})

	t.Run("Create panics if attempt to create a document from nil", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		assert.Panics(t, func() {
			_ = s.Create(ctx, "myNilID", nil)
		})
	})

	t.Run("Create does not store private fields", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		id := "create_private_fields"

		putxi := MakeSegador(
			withName("Putxi"),
		)
		putxi.internalField = "foobar"
		err := s.Create(ctx, id, putxi)
		require.NoError(t, err)

		var myPutxi Segador
		err = s.Get(ctx, id, &myPutxi)
		require.NoError(t, err)
		assert.Equal(t, "", myPutxi.internalField)
	})

	t.Run("Create always creates document with CreatedAt and UpdatedAt fields (struct)", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := "create_always_created_at_updated_at"

		err := s.Create(ctx, myID, myOtherEntity{})
		require.NoError(t, err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)

		assert.True(t, e1.CreatedAt.After(time.Now().Add(-time.Minute)))
		assert.True(t, e1.UpdatedAt.After(time.Now().Add(-time.Minute)))
	})
}

func testDatastoreSet(t *testing.T, serviceFactory FnServiceFactory) {
	ctx := context.Background()

	t.Run("Set returns error if data is not a struct or a map", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		assert.Error(t, s.Set(ctx, "my1234", 1234))

		var myReaper Reaper //nolint:gosimple
		myReaper = &bob
		assert.Error(t, s.Set(ctx, "my1234", &myReaper))
	})

	t.Run("Set with data argument with several levels of indirection", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		bobRef := &bob

		err := s.Set(ctx, "bobID", &bobRef)
		require.NoError(t, err)

		var myVal Segador
		err = s.Get(ctx, "bobID", &myVal)
		require.NoError(t, err)
		assert.EqualValues(t, bob, myVal)
	})

	t.Run("Set updates record if document already exists", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		err := s.Set(ctx, "NighthawkM1", bob)
		require.NoError(t, err)

		var myVal Segador
		err = s.Get(ctx, "NighthawkM1", &myVal)
		require.NoError(t, err)
		assert.EqualValues(t, bob, myVal)
	})

	t.Run("Set panics if attempt to create a document from nil", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		assert.Panics(t, func() {
			_ = s.Set(ctx, "myNilID", nil)
		})
	})

	t.Run("Set updates UpdatedAt field", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := "updates_updated_at"

		err := s.Set(ctx, myID, myOtherEntity{})
		require.NoError(t, err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)

		assert.True(t, e1.UpdatedAt.After(time.Now().Add(-time.Minute)))
	})
}

func testDatastoreGet(t *testing.T, serviceFactory FnServiceFactory) {
	ctx := context.Background()

	t.Run("Get retrieves a document", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := "retrieves_doc"

		var myBob Segador

		err := s.Get(ctx, myID, &myBob)
		assert.Equal(t, document.ErrNotFound, err)

		err = s.Create(ctx, myID, bob)
		require.NoError(t, err)

		err = s.Get(ctx, myID, &myBob)
		require.NoError(t, err)
		assert.EqualValues(t, bob, myBob)
	})

	t.Run("Get returns ErrNotFound if document does not exist", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		var myVal Segador
		require.Equal(t, document.ErrNotFound, s.Get(ctx, "bobID", &myVal))
	})

	t.Run("Get errors with anything that's not a pointer to struct or map", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		myID := "get_errors_non_ptr"
		err := s.Create(ctx, myID, myOtherEntity{})
		require.NoError(t, err)

		var myVal Segador
		require.Error(t, s.Get(ctx, myID, myVal))
	})

	t.Run("Get with map receiver", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := "get_map_receiver"

		err := s.Create(ctx, myID, bob)
		require.NoError(t, err)

		var myBob map[string]interface{}

		err = s.Get(ctx, myID, &myBob)
		require.NoError(t, err)

		bob.assertEqualMap(t, s, myBob)
	})
}

func testDatastoreDelete(t *testing.T, serviceFactory FnServiceFactory) {
	ctx := context.Background()

	t.Run("Get after a Delete returns nil", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := "get_delete_notfound"

		var myBob Segador

		err := s.Create(ctx, myID, bob)
		require.NoError(t, err)

		err = s.Get(ctx, myID, &myBob)
		require.NoError(t, err)

		err = s.Delete(ctx, myID)
		require.NoError(t, err)

		err = s.Get(ctx, myID, &myBob)
		require.Equal(t, document.ErrNotFound, err)
	})

	t.Run("Delete is idempotent", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := "get_delete_notfound"

		err := s.Create(ctx, myID, bob)
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			err = s.Delete(ctx, myID)
			require.NoError(t, err)
		}
	})

	t.Run("Delete for a document that doesn't exist returns no error", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := "get_delete_notfound"

		err := s.Delete(ctx, myID)
		require.NoError(t, err)
	})
}

func updateName(newName string) document.Update {
	return document.Update{
		FieldPath: []string{"Name"},
		Value:     newName,
	}
}

func updateTrait(k string, v interface{}) document.Update {
	return document.Update{
		FieldPath: []string{"Traits", k},
		Value:     v,
	}
}
func prepareForUpdate(
	t *testing.T, myID string, document interface{},
	serviceFactory FnServiceFactory,
) (s document.Service) {
	ctx := context.Background()
	s = serviceFactory(t)
	err := s.Create(ctx, myID, document)
	require.NoError(t, err)
	return
}

func testDatastoreUpdate(t *testing.T, serviceFactory FnServiceFactory) {

	ctx := context.Background()

	t.Run("Update panics if updates is empty", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := "update_panics"
		updates := make([]document.Update, 0)

		assert.Panics(t, func() {
			_ = s.Update(ctx, myID, updates)
		})
	})

	t.Run("Update returns ErrNotFound if attempting to update a document that does not exist", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := "update_errnotfound"
		updates := make([]document.Update, 1)
		updates[0].FieldPath = []string{"fjklewjflwk"}

		err := s.Update(ctx, myID, updates)
		require.Equal(t, document.ErrNotFound, err)
	})

	t.Run("Update updates a document field", func(t *testing.T) {
		myID := "updates_doc_field"
		s := prepareForUpdate(t, myID, alice, serviceFactory)
		defer s.Close()

		err := s.Update(ctx, myID, []document.Update{updateName("Alexandra")})
		require.NoError(t, err)

		var myNewAlice Segador
		err = s.Get(ctx, myID, &myNewAlice)
		require.NoError(t, err)
		assert.Equal(t, "Alexandra", myNewAlice.Name)
	})

	t.Run("Update updates a nested document field", func(t *testing.T) {
		myID := "update_nested_doc_field"
		s := prepareForUpdate(t, myID, alice, serviceFactory)
		defer s.Close()

		err := s.Update(ctx, myID, []document.Update{
			updateTrait("dob", "2017-03-44"),
		})
		require.NoError(t, err)

		var myNewAlice Segador
		err = s.Get(ctx, myID, &myNewAlice)
		require.NoError(t, err)
		assert.Equal(t, "2017-03-44", myNewAlice.Traits["dob"])
	})

	t.Run("Update converts a nested document field when type is a struct", func(t *testing.T) {
		myID := "converts_nested_doc_field_struct"
		s := prepareForUpdate(t, myID, alice, serviceFactory)
		defer s.Close()

		err := s.Update(ctx, myID, []document.Update{
			updateTrait("brother", bob),
		})
		require.NoError(t, err)

		var myNewAlice Segador
		err = s.Get(ctx, myID, &myNewAlice)
		require.NoError(t, err)

		bro := myNewAlice.Traits["brother"]
		require.NotNil(t, bro)

		require.Equal(t, reflect.Map, reflect.ValueOf(bro).Kind())
		bob.assertEqualMap(t, s, bro.(map[string]interface{}))
	})

	t.Run("Update DOES NOT update a nested document field that does not exist", func(t *testing.T) {
		myID := "update_not_update_nested_not_exist"
		s := prepareForUpdate(t, myID, alice, serviceFactory)
		defer s.Close()

		myNewAttr := make(map[string]interface{})
		myNewAttr["sup"] = "hola"

		err := s.Update(ctx, myID, []document.Update{
			updateTrait("myNewMap", myNewAttr),
		})
		require.NoError(t, err)

		var myNewAlice Segador
		err = s.Get(ctx, myID, &myNewAlice)
		require.NoError(t, err)
		assert.Equal(t, nil, myNewAlice.Traits["sup"])
	})

	t.Run("Update processes multiple updates", func(t *testing.T) {
		myID := "multiple_updates"
		s := prepareForUpdate(t, myID, alice, serviceFactory)
		defer s.Close()

		err := s.Update(ctx, myID, []document.Update{
			updateTrait("dob", "2020-02-21"),
			updateName("Alice"),
		})
		require.NoError(t, err)

		var myNewAlice Segador
		err = s.Get(ctx, myID, &myNewAlice)
		require.NoError(t, err)
		assert.Equal(t, "Alice", myNewAlice.Name)
		assert.Equal(t, "2020-02-21", myNewAlice.Traits["dob"])
	})

	t.Run("Update overrides client UpdatedAt field", func(t *testing.T) {
		myID := "update_overrides_updated_at"
		s := prepareForUpdate(t, myID, myOtherEntity{}, serviceFactory)
		defer s.Close()

		t1 := time.Now().Add(-time.Hour * 48)

		err := s.Update(ctx, myID, []document.Update{
			{FieldPath: []string{document.DefaultUpdatedAtField}, Value: t1},
		})
		require.NoError(t, err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)

		assert.NotEqual(t, t1, e1.UpdatedAt)
		assert.True(t, e1.UpdatedAt.After(time.Now().Add(-time.Minute)),
			"%v vs %v", e1.UpdatedAt, time.Now())
	})

	t.Run("Update always updates UpdatedAt field", func(t *testing.T) {
		myID := "update_always_updates_updated_at_field"
		s := prepareForUpdate(t, myID, myOtherEntity{}, serviceFactory)
		defer s.Close()

		err := s.Update(ctx, myID, []document.Update{
			{FieldPath: []string{"Value"}, Value: 1},
		})
		require.NoError(t, err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)

		assert.Equal(t, 1, e1.Value)
		assert.True(t, e1.UpdatedAt.After(time.Now().Add(-time.Minute)))
	})

	t.Run("Update fails if precondition is not met", func(t *testing.T) {
		myID := "update_precondition_updated_at"
		s := prepareForUpdate(t, myID, myOtherEntity{}, serviceFactory)
		defer s.Close()

		t1 := time.Now().Add(-time.Hour * 48)

		err := s.Update(ctx, myID,
			[]document.Update{
				{FieldPath: []string{"Value"}, Value: 1234},
			},
			document.Precondition{
				FieldPath: []string{document.DefaultUpdatedAtField}, Value: t1,
			},
		)
		require.Equal(t, document.ErrPreconditionFailed, err, "%v", err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)
		assert.Equal(t, 0, e1.Value)
	})
}

func prepareServiceForListTest(
	t *testing.T, name string, serviceFactory FnServiceFactory,
) document.Service {
	s := serviceFactory(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		myID := fmt.Sprintf("%s_list_bob_%d", name, i)
		err := s.Create(ctx, myID, bob)
		require.NoError(t, err)
	}

	for i := 0; i < 2; i++ {
		myID := fmt.Sprintf("%s_list_alice_%d", name, i)
		err := s.Create(ctx, myID, alice)
		require.NoError(t, err)
	}

	return s
}

func assertListResults(
	t *testing.T, it document.Iterator, expectedLen int,
) {
	var i int
	if expectedLen > 0 {
		// HasNext should be idempotent
		assert.True(t, it.HasNext())
	}

	for it.HasNext() {
		var s Segador
		err := it.NextTo(&s)
		require.NoError(t, err)
		i++
	}
	assert.False(t, it.HasNext())

	assert.Equal(t, expectedLen, i)
	assert.NoError(t, it.Close())
}

func testDatastoreList(t *testing.T, serviceFactory FnServiceFactory) {
	ctx := context.Background()

	t.Run("List retrieves ALL document if filters is nil or empty", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_all_doc_nil_filter", serviceFactory)
		defer s.Close()

		it, err := s.List(ctx, nil)
		require.NoError(t, err)

		assertListResults(t, it, 12)
	})

	t.Run("NextTo errors with anything that's not a pointer to struct or map", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_nextto_errors_anyting", serviceFactory)
		defer s.Close()

		var myVal Segador
		it, err := s.List(ctx, nil)
		require.NoError(t, err)
		assert.Error(t, it.NextTo(myVal))
		assert.NoError(t, it.Close())
	})

	t.Run("NextTo with map receiver", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_nextto_map_receiver", serviceFactory)
		defer s.Close()

		it, err := s.List(ctx, []document.Filter{nameFilter("Bob", document.OpEqual)})
		require.NoError(t, err)

		var myBob map[string]interface{}
		require.True(t, it.HasNext())
		err = it.NextTo(&myBob)
		require.NoError(t, err)

		bob.assertEqualMap(t, s, myBob)
		assert.NoError(t, it.Close())
	})

	t.Run("List zero-value filter panics", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_zero_filter_panics", serviceFactory)
		defer s.Close()

		filters := []document.Filter{{}}
		assert.Panics(t, func() {
			_, _ = s.List(ctx, filters)
		})
	})

	t.Run("List OpEqual filter retrieves of documents with prop equal to a value", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_op_equal", serviceFactory)
		defer s.Close()

		filters := []document.Filter{nameFilter("Alice", document.OpEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 2)
	})

	t.Run("List OpGreaterThan filter retrieves of documents with prop greater than a value", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_op_greater_than", serviceFactory)
		defer s.Close()

		filters := []document.Filter{nameFilter("B", document.OpGreaterThan)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 10)
	})

	t.Run("List OpGreaterThanEqual filter retrieves of documents with prop greater than or equal to value", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_op_greater_than_equal", serviceFactory)
		defer s.Close()

		filters := []document.Filter{nameFilter("Bob", document.OpGreaterThanEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 10)
	})

	t.Run("List OpLessThan filter retrieves of documents with prop less than a value", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_op_less_than", serviceFactory)
		defer s.Close()

		filters := []document.Filter{nameFilter("B", document.OpLessThan)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 2)
	})

	t.Run("List OpLessThanEqual filter retrieves of documents with prop less than or equal to a value", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_op_less_than_equal", serviceFactory)
		defer s.Close()

		filters := []document.Filter{nameFilter("Bob", document.OpLessThanEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 12)
	})

	t.Run("List with multiple filters retrieves of documents with AND filter predicate", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_multiple_filters_and_predicate", serviceFactory)
		defer s.Close()

		filters := []document.Filter{
			nameFilter("B", document.OpLessThan),
			nameFilter("B", document.OpGreaterThanEqual),
		}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 0)

		filters = []document.Filter{
			nameFilter("A", document.OpGreaterThan),
			nameFilter("Bob", document.OpLessThanEqual),
		}
		it, err = s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 12)
	})

	t.Run("List with filter on nested field", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_with_filter_nested_field", serviceFactory)
		defer s.Close()

		filters := []document.Filter{traitFilter("fancy", true, document.OpEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 10)
	})

	t.Run("List with filter with value incorrect numeric type coerces type", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_incorrect_numeric_coerces", serviceFactory)
		defer s.Close()

		filters := []document.Filter{traitFilter("years", int(30), document.OpGreaterThanEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 12)
	})

	t.Run("List with filter with value incorrect type returns nothing", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_with_filter_value_incorrect_type_returns_nothing", serviceFactory)
		defer s.Close()

		filters := []document.Filter{traitFilter("years", "30", document.OpEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 0)
	})

	t.Run("List with filter with incorrect field path returns nothing", func(t *testing.T) {
		s := prepareServiceForListTest(t, "list_incorrect_field_returns_nothing", serviceFactory)
		defer s.Close()

		filters := []document.Filter{traitFilter("wtf", "PROLLY", document.OpEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 0)
	})
}

func traitFilter(field string, value interface{}, op document.Op) document.Filter {
	return document.Filter{Field: document.Field{
		FieldPath: []string{"Traits", field},
		Value:     value,
	}, Op: op}
}

func nameFilter(value string, op document.Op) document.Filter {
	return document.Filter{Field: document.Field{
		FieldPath: []string{"Name"},
		Value:     value,
	}, Op: op}
}

// TestDocumentService runs an exhaustive suite of tests against a document.Service.
func TestDocumentService(t *testing.T, serviceFactory FnServiceFactory) {
	testDatastoreCreate(t, serviceFactory)
	testDatastoreSet(t, serviceFactory)
	testDatastoreGet(t, serviceFactory)
	testDatastoreDelete(t, serviceFactory)
	testDatastoreUpdate(t, serviceFactory)
	testDatastoreList(t, serviceFactory)

	t.Run("is threadsafe", func(t *testing.T) {
		ctx := context.Background()
		s := serviceFactory(t)
		defer s.Close()
		myID := "is_threadsafe"
		n := 10

		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				var myBob Segador
				_ = s.Create(ctx, myID, bob)
				_ = s.Get(ctx, myID, &myBob)
				_ = s.Update(ctx, myID, []document.Update{{FieldPath: []string{"Name"}, Value: "value"}})
				_ = s.Delete(ctx, myID)
				it, err := s.List(ctx, nil)
				if err == nil {
					it.Close()
				}
			}()
		}

		wg.Wait()

		// wait for eventually consistent implementations
		timeout := 2000 * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		retryStrategy := retry.SequentialStrategy(50 * time.Millisecond)
		err := retry.Retry(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
			var myBob Segador
			err := s.Get(ctx, myID, &myBob)
			if err != document.ErrNotFound {
				return true, fmt.Errorf("expected ErrNotFound found %v", err)
			}
			return false, nil
		})
		assert.NoError(t, err)
	})
}

// TestDocumentServicePreconditions allow implementations to to test preconditions
// separately as how and what preconditions work differs between implementations.
func TestDocumentServicePreconditions(t *testing.T, serviceFactory FnServiceFactory) {
	ctx := context.Background()
	t.Run("Update does not fails if updated time precondition is met", func(t *testing.T) {
		myID := "update_precondition_updated_at"
		s := prepareForUpdate(t, myID, myOtherEntity{}, serviceFactory)
		defer s.Close()

		var entity myOtherEntity
		// get updated timestamp
		require.NoError(t, s.Get(ctx, myID, &entity))
		require.NotZero(t, entity.UpdatedAt)

		err := s.Update(ctx, myID,
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

	t.Run("Update does not fails if version precondition is met", func(t *testing.T) {
		myID := "update_precondition_updated_at"
		entity := myOtherEntity{Value: 1}
		s := prepareForUpdate(t, myID, entity, serviceFactory)
		defer s.Close()

		err := s.Update(ctx, myID,
			[]document.Update{
				{FieldPath: []string{"Value"}, Value: 1234},
			},
			document.Precondition{
				FieldPath: []string{"Value"}, Value: 1,
			},
		)
		require.NoError(t, err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)
		assert.Equal(t, 1234, e1.Value)
	})

	t.Run("consistent updates", func(t *testing.T) {
		myID := "consistent_updates_id"
		type myStruct struct {
			Count   int
			Version int
		}
		s := prepareForUpdate(t, myID, myStruct{}, serviceFactory)
		defer s.Close()
		m := myStruct{}
		docID := uuid.New().String()
		ctx := context.Background()
		require.NoError(t, s.Create(ctx, docID, m))

		n := 100
		retryStrategy := retry.CombinedStrategy(
			retry.LimitStrategy(uint(n+1)),
		)
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(n)
		errors := make([]error, n)
		for i := 0; i < n; i++ {
			go func(m myStruct, i int) {
				defer wg.Done()
				<-start
				errors[i] = document.ConsistentUpdate(ctx, s, docID, &m, retryStrategy,
					func() ([]document.Update, []document.Precondition) {
						return []document.Update{
								{FieldPath: []string{"Count"}, Value: m.Count + 1},
								{FieldPath: []string{"Version"}, Value: m.Version + 1},
							}, []document.Precondition{
								{FieldPath: []string{"Version"}, Value: m.Version},
							}
					})
			}(m, i)
		}
		close(start)
		wg.Wait()
		var err error
		for _, nerr := range errors {
			if nerr != nil {
				err = multierror.Append(err, nerr)
			}
		}
		m = myStruct{}
		require.NoError(t, err)
		require.NoError(t, s.Get(ctx, docID, &m))
		assert.Equal(t, n, m.Count, m.Version)
	})
}

// TestDocumentService runs an exhaustive suite of tests against a document.Service,
// but omits any List tests.
func TestDocumentServiceNoList(t *testing.T, serviceFactory FnServiceFactory) {
	testDatastoreCreate(t, serviceFactory)
	testDatastoreSet(t, serviceFactory)
	testDatastoreGet(t, serviceFactory)
	testDatastoreDelete(t, serviceFactory)
	testDatastoreUpdate(t, serviceFactory)
}
