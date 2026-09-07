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

package debug

import (
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapturePanic(t *testing.T) {
	t.Run("allows func to complete successfully", func(t *testing.T) {
		var ok bool
		actualReport, panicValue, actualOk := CapturePanic(log.New(), "pkg", "v1.0.0", func() {
			ok = true
		})
		assert.Zero(t, panicValue)
		assert.True(t, actualOk)
		assert.Zero(t, actualReport)
		assert.True(t, ok)
	})

	t.Run("captures panic and returns report", func(t *testing.T) {
		actualReport, panicValue, actualOk := CapturePanic(log.New(), "pkg", "v1.0.0", func() {
			panic("ralfing")
		})
		assert.False(t, actualOk)
		stack, ok := actualReport.Metadata[reportMetadataStackTraceField]
		require.True(t, ok)
		assert.NotNil(t, stack)
		assert.NotZero(t, actualReport.Build)
		assert.NotZero(t, actualReport.CreatedAt)
		err, ok := actualReport.Metadata[reportMetadataErrorField]
		require.True(t, ok)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err, "ralfing"))
		assert.Equal(t, "ralfing", panicValue)
	})
}
