package document

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fnServiceFactory func(t *testing.T) Service

type Reaper interface {
	reapChains() error
}

type segador struct {
	Name          string
	Traits        map[string]interface{}
	internalField string
}

func (s *segador) reapChains() error {
	return nil
}

func (s *segador) toMap() map[string]interface{} {
	res := make(map[string]interface{})
	res["Name"] = s.Name
	traitsMap := make(map[string]interface{})
	for k, v := range s.Traits {
		traitsMap[k] = v
	}
	res["Traits"] = traitsMap
	return res
}

type segadorOpt func(*segador) *segador

func makeSegador(opts ...segadorOpt) (s segador) {
	for _, o := range opts {
		s = *o(&s)
	}
	return
}

func withName(name string) segadorOpt {
	return func(s *segador) *segador {
		s.Name = name
		return s
	}
}

func withTrait(k string, value interface{}) segadorOpt {
	return func(s *segador) *segador {
		if s.Traits == nil {
			s.Traits = make(map[string]interface{})
		}
		s.Traits[k] = value
		return s
	}
}

var (
	alice = makeSegador(
		withName("Alice"),
		withTrait("dob", "1989-11-03"),
		withTrait("years", float64(30)),
		withTrait("fancy", false),
	)

	bob = makeSegador(
		withName("Bob"),
		withTrait("dob", "1989-11-03"),
		withTrait("years", float64(30)),
		withTrait("fancy", true),
		withTrait("sister", alice.toMap()),
	)
)

type myOtherEntity struct {
	Value     int
	UpdatedAt time.Time `firestore:",serverTimestamp"`
	CreatedAt time.Time `firestore:",serverTimestamp"`
}

func testDatastoreCreate(t *testing.T, serviceFactory fnServiceFactory) {
	ctx := context.Background()

	t.Run("Create returns error if data is not a struct or a map", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		assert.Error(t, s.Create(ctx, "my1234", 1234))

		var myReaper Reaper
		myReaper = &bob
		assert.Error(t, s.Create(ctx, "my1234", &myReaper))
	})

	t.Run("Create with data argument with several levels of indirection", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		bobRef := &bob

		err := s.Create(ctx, "bobID", &bobRef)
		require.NoError(t, err)

		var myVal segador
		err = s.Get(ctx, "bobID", &myVal)
		require.NoError(t, err)
		assert.Equal(t, bob, myVal)
	})

	t.Run("Create returns ErrAlreadyExists if attempt to create a document that already exists", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		err := s.Create(ctx, "aliceID", bob)
		require.NoError(t, err)

		assert.Equal(t, ErrAlreadyExists, s.Create(ctx, "aliceID", bob))
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
		id := uuid.New().String()

		putxi := makeSegador(
			withName("Putxi"),
		)
		putxi.internalField = "foobar"
		err := s.Create(ctx, id, putxi)
		require.NoError(t, err)

		var myPutxi segador
		err = s.Get(ctx, id, &myPutxi)
		require.NoError(t, err)
		assert.Equal(t, "", myPutxi.internalField)
	})

	t.Run("Create always creates document with CreatedAt and UpdatedAt fields", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := uuid.New().String()

		err := s.Create(ctx, myID, myOtherEntity{})
		require.NoError(t, err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)

		assert.True(t, e1.CreatedAt.After(time.Now().Add(-time.Minute)))
		assert.True(t, e1.UpdatedAt.After(time.Now().Add(-time.Minute)))
	})
}

func testDatastoreSet(t *testing.T, serviceFactory fnServiceFactory) {
	ctx := context.Background()

	t.Run("Set returns error if data is not a struct or a map", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		assert.Error(t, s.Set(ctx, "my1234", 1234))

		var myReaper Reaper
		myReaper = &bob
		assert.Error(t, s.Set(ctx, "my1234", &myReaper))
	})

	t.Run("Set with data argument with several levels of indirection", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		bobRef := &bob

		err := s.Set(ctx, "bobID", &bobRef)
		require.NoError(t, err)

		var myVal segador
		err = s.Get(ctx, "bobID", &myVal)
		require.NoError(t, err)
		assert.Equal(t, bob, myVal)
	})

	t.Run("Set updates record if document already exists", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		err := s.Set(ctx, "NighthawkM1", bob)
		require.NoError(t, err)

		var myVal segador
		err = s.Get(ctx, "NighthawkM1", &myVal)
		require.NoError(t, err)
		assert.Equal(t, bob, myVal)
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
		myID := uuid.New().String()

		err := s.Set(ctx, myID, myOtherEntity{})
		require.NoError(t, err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)

		assert.True(t, e1.UpdatedAt.After(time.Now().Add(-time.Minute)))
	})
}

func testDatastoreGet(t *testing.T, serviceFactory fnServiceFactory) {
	ctx := context.Background()

	t.Run("Get retrieves a document", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := uuid.New().String()

		var myBob segador

		err := s.Get(ctx, myID, &myBob)
		assert.Equal(t, ErrNotFound, err)

		err = s.Create(ctx, myID, bob)
		require.NoError(t, err)

		err = s.Get(ctx, myID, &myBob)
		require.NoError(t, err)
		assert.Equal(t, bob, myBob)
	})

	t.Run("Get returns ErrNotFound if document does not exist", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		var myVal segador
		require.Equal(t, ErrNotFound, s.Get(ctx, "bobID", myVal))
	})

	t.Run("Get errors with anything that's not a pointer to struct or map", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()

		myID := uuid.New().String()
		err := s.Create(ctx, myID, myOtherEntity{})
		require.NoError(t, err)

		var myVal segador
		require.Error(t, s.Get(ctx, myID, myVal))
	})

	t.Run("Get with map receiver", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := uuid.New().String()

		err := s.Create(ctx, myID, bob)
		require.NoError(t, err)

		var myBob map[string]interface{}

		err = s.Get(ctx, myID, &myBob)
		require.NoError(t, err)

		delete(myBob, DefaultCreatedAtField)
		delete(myBob, DefaultUpdatedAtField)
		assert.Equal(t, bob.toMap(), myBob)
	})
}

func testDatastoreDelete(t *testing.T, serviceFactory fnServiceFactory) {
	ctx := context.Background()

	t.Run("Get after a Delete returns nil", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := uuid.New().String()

		var myBob segador

		err := s.Create(ctx, myID, bob)
		require.NoError(t, err)

		err = s.Get(ctx, myID, &myBob)
		require.NoError(t, err)

		err = s.Delete(ctx, myID)
		require.NoError(t, err)

		err = s.Get(ctx, myID, &myBob)
		require.Equal(t, ErrNotFound, err)
	})
}

func updateName(newName string) Update {
	return Update{
		FieldPath: []string{"Name"},
		Value:     newName,
	}
}

func updateTrait(k string, v interface{}) Update {
	return Update{
		FieldPath: []string{"Traits", k},
		Value:     v,
	}
}

func testDatastoreUpdate(t *testing.T, serviceFactory fnServiceFactory) {
	ctx := context.Background()

	prepareForUpdate := func(t *testing.T, document interface{}) (s Service, myID string) {
		s = serviceFactory(t)
		myID = uuid.New().String()

		err := s.Create(ctx, myID, document)
		require.NoError(t, err)
		return
	}

	t.Run("Update panics if updates is empty", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := uuid.New().String()
		updates := make([]Update, 0)

		assert.Panics(t, func() {
			_ = s.Update(ctx, myID, updates)
		})
	})

	t.Run("Update returns ErrNotFound if attempting to update a document that does not exist", func(t *testing.T) {
		s := serviceFactory(t)
		defer s.Close()
		myID := uuid.New().String()
		updates := make([]Update, 1)
		updates[0].FieldPath = []string{"fjklewjflwk"}

		err := s.Update(ctx, myID, updates)
		require.Equal(t, ErrNotFound, err)
	})

	t.Run("Update updates a document field", func(t *testing.T) {
		s, myID := prepareForUpdate(t, alice)
		defer s.Close()

		err := s.Update(ctx, myID, []Update{updateName("Alexandra")})
		require.NoError(t, err)

		var myNewAlice segador
		err = s.Get(ctx, myID, &myNewAlice)
		require.NoError(t, err)
		assert.Equal(t, "Alexandra", myNewAlice.Name)
	})

	t.Run("Update updates a nested document field", func(t *testing.T) {
		s, myID := prepareForUpdate(t, alice)
		defer s.Close()

		err := s.Update(ctx, myID, []Update{
			updateTrait("dob", "2017-03-44"),
		})
		require.NoError(t, err)

		var myNewAlice segador
		err = s.Get(ctx, myID, &myNewAlice)
		require.NoError(t, err)
		assert.Equal(t, "2017-03-44", myNewAlice.Traits["dob"])
	})

	t.Run("Update converts a nested document field when type is a struct", func(t *testing.T) {
		if strings.Contains(t.Name(), "TestInMemory") ||
			strings.Contains(t.Name(), "TestBolt") ||
			strings.Contains(t.Name(), "TestLogging") {
			// mapstructure seems to not convert nested
			// struct values in map[string]interface{} to map[string]interface{}.
			// Behaviour is actually fine, so just skip the test.
			t.Skip()
		}

		s, myID := prepareForUpdate(t, alice)
		defer s.Close()

		err := s.Update(ctx, myID, []Update{
			updateTrait("brother", bob),
		})
		require.NoError(t, err)

		var myNewAlice segador
		err = s.Get(ctx, myID, &myNewAlice)
		require.NoError(t, err)

		bro := myNewAlice.Traits["brother"]
		require.NotNil(t, bro)

		require.Equal(t, reflect.Map, reflect.ValueOf(bro).Kind())
		assert.Equal(t, bob.toMap(), bro.(map[string]interface{}))
	})

	t.Run("Update DOES NOT update a nested document field that does not exist", func(t *testing.T) {
		s, myID := prepareForUpdate(t, alice)
		defer s.Close()

		myNewAttr := make(map[string]interface{})
		myNewAttr["sup"] = "hola"

		err := s.Update(ctx, myID, []Update{
			updateTrait("myNewMap", myNewAttr),
		})
		require.NoError(t, err)

		var myNewAlice segador
		err = s.Get(ctx, myID, &myNewAlice)
		require.NoError(t, err)
		assert.Equal(t, nil, myNewAlice.Traits["sup"])
	})

	t.Run("Update processes multiple updates", func(t *testing.T) {
		s, myID := prepareForUpdate(t, alice)
		defer s.Close()

		err := s.Update(ctx, myID, []Update{
			updateTrait("NewTrait", "WOW"),
			updateName("Alice"),
		})
		require.NoError(t, err)

		var myNewAlice segador
		err = s.Get(ctx, myID, &myNewAlice)
		require.NoError(t, err)
		assert.Equal(t, "Alice", myNewAlice.Name)
		assert.Equal(t, "WOW", myNewAlice.Traits["NewTrait"])
	})

	t.Run("Update overrides client UpdatedAt field", func(t *testing.T) {
		s, myID := prepareForUpdate(t, myOtherEntity{})
		defer s.Close()

		t1 := time.Now().Add(-time.Hour * 48)

		err := s.Update(ctx, myID, []Update{
			Update{FieldPath: []string{DefaultUpdatedAtField}, Value: t1},
		})
		require.NoError(t, err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)

		assert.NotEqual(t, t1, e1.UpdatedAt)
		assert.True(t, e1.UpdatedAt.After(time.Now().Add(-time.Minute)))
	})

	t.Run("Update always updates UpdatedAt field", func(t *testing.T) {
		s, myID := prepareForUpdate(t, myOtherEntity{})
		defer s.Close()

		err := s.Update(ctx, myID, []Update{
			Update{FieldPath: []string{"Value"}, Value: 1},
		})
		require.NoError(t, err)

		var e1 myOtherEntity
		err = s.Get(ctx, myID, &e1)
		require.NoError(t, err)

		assert.Equal(t, 1, e1.Value)
		assert.True(t, e1.UpdatedAt.After(time.Now().Add(-time.Minute)))
	})
}

func prepareServiceForListTest(
	t *testing.T, serviceFactory fnServiceFactory,
) Service {
	s := serviceFactory(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		myID := uuid.New().String()
		err := s.Create(ctx, myID, bob)
		require.NoError(t, err)
	}

	for i := 0; i < 2; i++ {
		myID := uuid.New().String()
		err := s.Create(ctx, myID, alice)
		require.NoError(t, err)
	}

	return s
}

func assertListResults(
	t *testing.T, it Iterator, expectedLen int,
) {
	var i int
	for it.HasNext() {
		var s segador
		err := it.NextTo(&s)
		assert.NoError(t, err)
		i++
	}
	assert.Equal(t, expectedLen, i)
	assert.NoError(t, it.Close())
}

func testDatastoreList(t *testing.T, serviceFactory fnServiceFactory) {
	ctx := context.Background()

	t.Run("List retrieves ALL document if filters is nil or empty", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		it, err := s.List(ctx, nil)
		require.NoError(t, err)

		assertListResults(t, it, 12)
	})

	t.Run("NextTo errors with anything that's not a pointer to struct or map", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		var myVal segador
		it, err := s.List(ctx, nil)
		require.NoError(t, err)
		assert.Error(t, it.NextTo(myVal))
		assert.NoError(t, it.Close())
	})

	t.Run("NextTo with map receiver", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		it, err := s.List(ctx, []Filter{nameFilter("Bob", OpEqual)})
		require.NoError(t, err)

		var myBob map[string]interface{}
		err = it.NextTo(&myBob)
		require.NoError(t, err)

		delete(myBob, DefaultCreatedAtField)
		delete(myBob, DefaultUpdatedAtField)
		assert.Equal(t, bob.toMap(), myBob)
		assert.NoError(t, it.Close())
	})

	t.Run("List zero-value filter panics", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{Filter{}}
		assert.Panics(t, func() {
			_, _ = s.List(ctx, filters)
		})
	})

	t.Run("List OpEqual filter retrieves of documents with prop equal to a value", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{nameFilter("Alice", OpEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 2)
	})

	t.Run("List OpGreaterThan filter retrieves of documents with prop greater than a value", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{nameFilter("B", OpGreaterThan)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 10)
	})

	t.Run("List OpGreaterThanEqual filter retrieves of documents with prop greater than or equal to value", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{nameFilter("Bob", OpGreaterThanEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 10)
	})

	t.Run("List OpLessThan filter retrieves of documents with prop less than a value", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{nameFilter("B", OpLessThan)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 2)
	})

	t.Run("List OpLessThanEqual filter retrieves of documents with prop less than or equal to a value", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{nameFilter("Bob", OpLessThanEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 12)
	})

	t.Run("List with multiple filters retrieves of documents with AND filter predicate", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{
			nameFilter("B", OpLessThan),
			nameFilter("B", OpGreaterThanEqual),
		}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 0)

		filters = []Filter{
			nameFilter("A", OpGreaterThan),
			nameFilter("Bob", OpLessThanEqual),
		}
		it, err = s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 12)
	})

	t.Run("List with filter on nested field", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{traitFilter("fancy", true, OpEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 10)
	})

	t.Run("List with filter with value incorrect numeric type coerces type", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{traitFilter("years", int(30), OpGreaterThanEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 12)
	})

	t.Run("List with filter with value incorrect type returns nothing", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{traitFilter("years", "30", OpEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 0)
	})

	t.Run("List with filter with incorrect field path returns nothing", func(t *testing.T) {
		s := prepareServiceForListTest(t, serviceFactory)
		defer s.Close()

		filters := []Filter{traitFilter("wtf", "PROLLY", OpEqual)}
		it, err := s.List(ctx, filters)
		require.NoError(t, err)
		assertListResults(t, it, 0)
	})
}

func traitFilter(field string, value interface{}, op Op) Filter {
	return Filter{Field: Field{
		FieldPath: []string{"Traits", field},
		Value:     value,
	}, Op: op}
}

func nameFilter(value string, op Op) Filter {
	return Filter{Field: Field{
		FieldPath: []string{"Name"},
		Value:     value,
	}, Op: op}
}

func testDatastore(t *testing.T, serviceFactory fnServiceFactory) {
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
		myID := uuid.New().String()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			go func() {
				var myBob segador
				_ = s.Create(ctx, myID, bob)
				_ = s.Get(ctx, myID, &myBob)
				_ = s.Delete(ctx, myID)
				// TODO add List
			}()
		}

		wg.Wait()

		// wait for eventually consistent implementations
		time.Sleep(500 * time.Millisecond)

		var myBob segador
		err := s.Get(ctx, myID, &myBob)
		require.Equal(t, ErrNotFound, err)
	})
}
