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
