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
