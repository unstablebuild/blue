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
		stack, ok := actualReport.Metadata[reportMetadataStackTraceField]
		require.True(t, ok)
		assert.NotNil(t, stack)
		assert.NotZero(t, actualReport.Build)
		assert.NotZero(t, actualReport.CreatedAt)
		err, ok := actualReport.Metadata[reportMetadataErrorField]
		require.True(t, ok)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err, "ralfing"))
	})
}
