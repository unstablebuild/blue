package secretmanager

import (
	"context"

	"github.com/ernestrc/blue/auth"
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

	return auth.SymmetricKey(sv.Payload), nil
}

func (s secretManagerKeys) Verify(ctx context.Context) ([]auth.Key, error) {
	svs, err := s.svc.AccessSecretVersions(ctx, s.secretID)
	if err != nil {
		return nil, err
	}

	var ret []auth.Key
	for _, sv := range svs {
		ret = append(ret, auth.SymmetricKey(sv.Payload))
	}
	return ret, nil
}
