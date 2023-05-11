package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/bson"
	"github.com/ernestrc/blue/encoding/json"
	"github.com/ernestrc/blue/encoding/toml"
	"github.com/stretchr/testify/require"
)

func TestPartitionService(t *testing.T) {
	tsuite := []struct {
		encoding  string
		marshaler encoding.Marshaler
	}{
		{"bson", bson.Marshaler()},
		{"json", json.Marshaler()},
		{"toml", toml.Marshaler()},
	}
	for _, tcase := range tsuite {
		t.Run("a single, default partition", func(t *testing.T) {
			t.Run(tcase.encoding, func(t *testing.T) {
				TestDocumentService(t, func(t *testing.T) document.Service {
					other := document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
					return document.WithPartition(other, "default")
				})
			})
		})

		t.Run("multiple partitions", func(t *testing.T) {
			t.Run(tcase.encoding, func(t *testing.T) {
				other := document.NewInMemoryServiceWithMarshaler(tcase.marshaler)
				var i int
				TestDocumentService(t, func(t *testing.T) document.Service {
					i++
					return document.WithPartition(other, fmt.Sprintf("%dth", i))
				})
			})
		})
	}

	// simple tests to make debuggin easier, but complete test is above in "multiple partitions"
	t.Run("records created by one are not seend by another", func(t *testing.T) {
		other := document.NewInMemoryService()
		one := document.WithPartition(other, "one")
		two := document.WithPartition(other, "two")

		type myStruct struct {
			A string
		}
		ctx := context.Background()
		require.NoError(t, one.Create(ctx, "myID", myStruct{A: "a"}))
		require.NoError(t, two.Create(ctx, "myID", myStruct{A: "a"}))

		it, err := one.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it, 1)

		it2, err := two.List(ctx, nil)
		require.NoError(t, err)
		assertListResults(t, it2, 1)
	})
}
