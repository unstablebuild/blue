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
		stack, ok := actualReport.Metadata[ReportMetadataStackTraceField]
		require.True(t, ok)
		assert.NotNil(t, stack)
		assert.NotZero(t, actualReport.Build)
		assert.NotZero(t, actualReport.CreatedAt)
		err, ok := actualReport.Metadata[ReportMetadataErrorField]
		require.True(t, ok)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err, "ralfing"))
	})
}
