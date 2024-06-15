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
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchPublicJWKS(t *testing.T) {
	// NOTE: to make this test work, we should create an http endpoint and return a pub key
	t.SkipNow()

	endpoint := "https://www.googleapis.com/oauth2/v3/certs"
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	keys, err := FetchPublicJWKS(u)
	require.NoError(t, err)

	verifyKeys, err := keys.Verify(context.Background())
	require.NoError(t, err)

	require.True(t, len(verifyKeys) >= 1)

	token := "REDACTED"

	for _, key := range verifyKeys {
		_, err = VerifyToken[any](key, token)
		if err == nil {
			break
		}
	}
	require.NoError(t, err)
}
