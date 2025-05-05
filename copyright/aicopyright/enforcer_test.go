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

package aicopyright

import (
	"context"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfringes(t *testing.T) {
	t.SkipNow() // do not test this all the time, as it consumes tokens
	openAiKey := os.Getenv("OPENAI_TESTING_KEY")

	t.Run("detects if image infringes copyright", func(t *testing.T) {
		enforcer := NewOpenaiEnforcer(openAiKey)

		img := loadTestImage(t, "./testdata/copyrighted_image.png")
		infringes, err := enforcer.ImageInfringes(context.Background(), img)
		require.NoError(t, err)

		assert.True(t, infringes)
	})

	t.Run("detects if image does not infringe copyright", func(t *testing.T) {
		enforcer := NewOpenaiEnforcer(openAiKey)

		img := loadTestImage(t, "./testdata/non_copyrighted_image.png")
		infringes, err := enforcer.ImageInfringes(context.Background(), img)
		require.NoError(t, err)

		assert.False(t, infringes)
	})
}

func loadTestImage(t *testing.T, image string) image.Image {
	f, err := os.Open(image)
	require.NoError(t, err)
	img, err := png.Decode(f)
	require.NoError(t, err)
	return img
}
