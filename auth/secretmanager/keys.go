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

package secretmanager

import (
	"context"

	"github.com/unstablebuild/blue/auth"
)

// SymmetricKeys returns a secretmanager.Service-backed implementation of auth.Keys.
// All the keys provided by the returned auth.Keys are symmetrical.
func SymmetricKeys(projectID, credsFile, secretID string) (auth.Keys, error) {
	svc, err := NewService(projectID, credsFile)
	if err != nil {
		return nil, err
	}
	return secretManagerKeys{svc: svc, secretID: secretID}, nil
}

type secretManagerKeys struct {
	svc      *Service
	secretID string
}

func (s secretManagerKeys) Sign(ctx context.Context) (auth.Key, error) {
	sv, err := s.svc.AccessSecretLatest(ctx, s.secretID)
	if err != nil {
		return auth.Key{}, err
	}

	return auth.SymmetricKey(sv.Payload)
}

func (s secretManagerKeys) Verify(ctx context.Context) ([]auth.Key, error) {
	svs, err := s.svc.AccessSecretVersions(ctx, s.secretID)
	if err != nil {
		return nil, err
	}

	var ret []auth.Key

	for _, sv := range svs {
		key, err := auth.SymmetricKey(sv.Payload)
		if err != nil {
			return nil, err
		}
		ret = append(ret, key)
	}
	return ret, nil
}
