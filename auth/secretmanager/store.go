package secretmanager

import (
	"context"

	"github.com/ernestrc/blue/auth"
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
