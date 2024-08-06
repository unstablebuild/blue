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

// SecretStore wraps returns an auth.SecretStore backed by secretmanager.Service.
func SecretStore(projectID, credsFile string) (auth.SecretStore, error) {
	svc, err := NewService(projectID, credsFile)
	if err != nil {
		return nil, err
	}
	return secretManagerStore{svc: svc}, nil
}

type secretManagerStore struct {
	svc *Service
}

func (s secretManagerStore) GetSecretMetadata(ctx context.Context, ID string) (map[string]string, error) {
	sec, err := s.svc.GetSecret(ctx, ID)
	if err != nil {
		return nil, err
	}
	return sec.Annotations, nil
}

func (s secretManagerStore) AccessSecret(ctx context.Context, ID string) ([]byte, error) {
	ver, err := s.svc.AccessSecretLatest(ctx, ID)
	if err != nil {
		return nil, err
	}
	return ver.Payload, nil
}
