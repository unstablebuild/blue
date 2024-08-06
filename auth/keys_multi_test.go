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

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombineKeys(t *testing.T) {
	t.Run("with two args", func(t *testing.T) {
		testSignKey1 := SymmetricKey([]byte("1234"))
		testSignKeys1 := StaticSymmetricKeys(testSignKey1)
		testSignKey2 := SymmetricKey([]byte("1235"))
		testSignKeys2 := StaticSymmetricKeys(testSignKey2)
		combined := CombineKeys(testSignKeys1, testSignKeys2)

		t.Run("Sign returns the first key arg", func(t *testing.T) {
			k, err := combined.Sign(context.Background())
			require.NoError(t, err)
			assert.Equal(t, testSignKey1, k)
		})

		t.Run("Verify returns all the keys", func(t *testing.T) {
			keys, err := combined.Verify(context.Background())
			require.NoError(t, err)
			assert.ElementsMatch(t, []Key{testSignKey1, testSignKey2}, keys)
		})
	})
	t.Run("with one key", func(t *testing.T) {
		testSignKey1 := SymmetricKey([]byte("1234"))
		testSignKeys1 := StaticSymmetricKeys(testSignKey1)
		combined := CombineKeys(testSignKeys1)

		t.Run("Sign returns the first key arg", func(t *testing.T) {
			k, err := combined.Sign(context.Background())
			require.NoError(t, err)
			assert.Equal(t, testSignKey1, k)
		})

		t.Run("Verify returns all the keys", func(t *testing.T) {
			keys, err := combined.Verify(context.Background())
			require.NoError(t, err)
			assert.ElementsMatch(t, []Key{testSignKey1}, keys)
		})
	})
}
