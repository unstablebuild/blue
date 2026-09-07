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

package auth

import (
	"context"
	"encoding/base64"
	"errors"
)

// EncodeSecretID encodes the given ID to be compatible with
// SecretStore implementations.
func EncodeSecretID(id string) string {
	return base64.StdEncoding.WithPadding(base64.NoPadding).
		EncodeToString([]byte(id))
}

// SecretStore abstracts the ability to retrieve secret metadata and access secret data.
type SecretStore interface {
	GetSecretMetadata(ctx context.Context, ID string) (map[string]string, error)
	AccessSecret(ctx context.Context, ID string) ([]byte, error)
}

// MapSecretStore returns a SecretStore of secrets with metadata.
// The argument metadata is expected to be pairs of key values and it is
// returned to all secrets.
func MapSecretStore(data map[string][]byte, metadata ...string) SecretStore {
	metadataMap := make(map[string]string)
	var prevKv string
	for i, kv := range metadata {
		if i%2 == 0 {
			prevKv = kv
			metadataMap[kv] = ""
		} else {
			metadataMap[prevKv] = kv
		}
	}
	return mapSecretStore{data: data, metadata: metadataMap}
}

type mapSecretStore struct {
	data     map[string][]byte
	metadata map[string]string
}

func (m mapSecretStore) GetSecretMetadata(ctx context.Context, ID string) (map[string]string, error) {
	ret := make(map[string]string, len(m.metadata))
	for k, v := range m.metadata {
		ret[k] = v
	}
	return ret, nil
}

func (m mapSecretStore) AccessSecret(ctx context.Context, ID string) ([]byte, error) {
	data, ok := m.data[ID]
	if !ok {
		return nil, errors.New("secret not found")
	}
	return data, nil
}
