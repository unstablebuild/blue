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

	token := "eyJhbGciOiJSUzI1NiIsImtpZCI6IjZmNzI1NDEwMWY1NmU0MWNmMzVjOTkyNmRlODRhMmQ1NTJiNGM2ZjEiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovL2FjY291bnRzLmdvb2dsZS5jb20iLCJhenAiOiI5MDU2NzExNjk3NjQtMzV0cXN2ODFpMHBzZTZhNmprMXI1NjR1YjZ1ZW4yNHQuYXBwcy5nb29nbGV1c2VyY29udGVudC5jb20iLCJhdWQiOiI5MDU2NzExNjk3NjQtMzV0cXN2ODFpMHBzZTZhNmprMXI1NjR1YjZ1ZW4yNHQuYXBwcy5nb29nbGV1c2VyY29udGVudC5jb20iLCJzdWIiOiIxMTQ3NjAzMjEyMjQ4NDYwMDk1NDAiLCJoZCI6InVuc3RhYmxlLmJ1aWxkIiwiZW1haWwiOiJlcm5lc3RAdW5zdGFibGUuYnVpbGQiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwibmJmIjoxNjk1NTAxOTcxLCJuYW1lIjoiRXJuZXN0IFJvbWVybyBDbGltZW50IiwicGljdHVyZSI6Imh0dHBzOi8vbGgzLmdvb2dsZXVzZXJjb250ZW50LmNvbS9hL0FDZzhvY0p6ZURsRWVkSU5oMVZGdVE0MW9CRmhpY01YamNGdmFDakxFTFdBcS0yRU1ST2I9czk2LWMiLCJnaXZlbl9uYW1lIjoiRXJuZXN0IiwiZmFtaWx5X25hbWUiOiJSb21lcm8gQ2xpbWVudCIsImxvY2FsZSI6ImVuIiwiaWF0IjoxNjk1NTAyMjcxLCJleHAiOjE2OTU1MDU4NzEsImp0aSI6IjhmMmJkZWY5ZWJiNGM4ODk1YWY2YjQ0ZjMyODAzNjMzOTdhN2QyNjUifQ.T_pazt3bgxZ6ttxJoVf_4k2SJgdJ7jOddz_rbS1trcivo2SKPcz3aLV4QNfX-_2bUa2sXLXoEebvDUpAhE18-OyJhQJvpErecGVxsWYCm43Qk4tL60av5eWFYGKEtZtr9g1Bmdb0WdZnytvXMKLEqQxl-JhpdXe6XNLBUFDK33A8NG4BgHXj-kDwsJVwukIzFWyVWaZ6aUUMhaKjs8ok1ZNsMWXa_asaykj-ApnSN9eUX4PMkq0il9Lxf9-I6s94_9psAJPNMZUyGHWgJGOaQG8T7RDOLEI91XvjtJRb6_sWbVDp0vd1ksqYuHgTKoguMbocc2acBICUJy6G2oGD5w"

	for _, key := range verifyKeys {
		_, err = VerifyToken[any](key, token)
		if err == nil {
			break
		}
	}
	require.NoError(t, err)
}
