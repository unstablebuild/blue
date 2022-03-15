package datastore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestTimestampConversion(t *testing.T) {
	now := time.Now()
	protoNow := StdTimeToProto(now)
	now2 := ProtoTimeToStd(&protoNow)
	protoNow2 := StdTimeToProto(now2)

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
