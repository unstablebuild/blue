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

package crypto

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/crypto/cryptotest"
)

func TestSignVerify(t *testing.T) {
	t.Run("verifies signature correctly", func(t *testing.T) {
		key := Key(cryptotest.GenerateTestKey(t))

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, key)
		require.NoError(t, err)

		err = Verify(strings.NewReader(str), &out, key)
		assert.NoError(t, err)
	})

	t.Run("returns error if data has been tampered with", func(t *testing.T) {
		key := Key(cryptotest.GenerateTestKey(t))

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, key)
		require.NoError(t, err)

		str = "bluectl cryp\x00"
		err = Verify(strings.NewReader(str), &out, key)
		assert.Error(t, err)
	})

	t.Run("returns error if a different key pair has been used to sign data", func(t *testing.T) {
		key1 := Key(cryptotest.GenerateTestKey(t))
		key2 := Key(cryptotest.GenerateTestKey(t))

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, key1)
		require.NoError(t, err)

		err = Verify(strings.NewReader(str), &out, key2)
		assert.Error(t, err)
	})
}
