package datastore

import (
	"context"
	"testing"
	"time"

	"github.com/ernestrc/blue/datastore/document"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/stretchr/testify/assert"
)

func TestTimestampConversion(t *testing.T) {
	now := time.Now()
	protoNow := stdTimeToProto(now)
	now2 := protoTimeToStd(&protoNow)
	protoNow2 := stdTimeToProto(now2)

	assert.Equal(t, now.UnixNano(), now2.UnixNano())
	assert.Equal(t, protoNow, protoNow2)
}

func testProtoStructConversion(t *testing.T, in map[string]*structpb.Value) {
	out := protoMapToStdMap(in)
	in2 := stdMapToProtoMap(out)
	out2 := protoMapToStdMap(in2)
	assert.Equal(t, in, in2)
	assert.Equal(t, out, out2)
}

func TestProtoStructConversion(t *testing.T) {

	t.Run("nil map", func(t *testing.T) {
		assert.Equal(t, make(map[string]interface{}), protoMapToStdMap(nil))
		assert.Equal(t, make(map[string]*structpb.Value), stdMapToProtoMap(nil))
	})

	t.Run("empty map", func(t *testing.T) {
		in := make(map[string]*structpb.Value)
		testProtoStructConversion(t, in)
	})

	t.Run("non empty map", func(t *testing.T) {
		testProtoStructConversion(t, fields1)
	})
}

func testResourcePersistence(
	t *testing.T, in, out Entity, validate func(*testing.T, Entity, Entity),
) {
	svc := document.NewInMemoryCache()

	assert.NoError(t, svc.Create(context.Background(), "1234", in))

	assert.NoError(t, svc.Get(context.Background(), "1234", out))
	validate(t, in, out)
}
