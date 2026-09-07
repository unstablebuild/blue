// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
