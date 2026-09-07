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
