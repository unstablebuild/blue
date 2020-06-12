package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newParsedFlagSet(t *testing.T) *FlagSet {
	fs := NewFlagSet("beteve")
	fs.Bool("reapers", true, "Revolution")
	fs.Bool("police", true, "Repression")

	err := fs.Parse([]string{"-reapers", "-police"})
	require.NoError(t, err)

	return fs
}

func TestPopulateContext(t *testing.T) {
	ctx := context.Background()
	fs := newParsedFlagSet(t)

	ctx = ContextWithOptions(ctx, fs)

	reapers, ok1 := OptionFromContext(ctx, "reapers")
	police, ok2 := OptionFromContext(ctx, "police")

	assert.Equal(t, true, ok1)
	assert.Equal(t, true, ok2)
	assert.Equal(t, true, reapers)
	assert.Equal(t, true, police)
}

func TestDoesNotConflict(t *testing.T) {
	ctx := context.Background()
	fs := newParsedFlagSet(t)

	ctx = ContextWithOptions(ctx, fs)

	type myKey string
	var reaper myKey = "reapers"
	var sickles myKey = "sickles"
	ctx = context.WithValue(ctx, reaper, false)
	ctx = context.WithValue(ctx, sickles, true)

	reapers, _ := OptionFromContext(ctx, "reapers")
	assert.Equal(t, true, reapers)

	_, ok := OptionFromContext(ctx, "sickles")
	assert.Equal(t, false, ok)
}
