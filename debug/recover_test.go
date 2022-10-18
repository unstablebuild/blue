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
		actualOk, actualReport := CapturePanic(log.New(), "pkg", "v1.0.0", func() {
			ok = true
		})
		assert.True(t, actualOk)
		assert.Zero(t, actualReport)
		assert.True(t, ok)
	})

	t.Run("captures panic and returns report", func(t *testing.T) {
		actualOk, actualReport := CapturePanic(log.New(), "pkg", "v1.0.0", func() {
			panic("ralfing")
		})
		assert.False(t, actualOk)
		assert.NotNil(t, actualReport.Stack)
		assert.NotZero(t, actualReport.BuildInfo)
		assert.NotZero(t, actualReport.CreatedAt)
		require.NotNil(t, actualReport.Error)
		assert.True(t, strings.Contains(actualReport.Error, "ralfing"))
	})
}
