package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	"golang.org/x/crypto/bcrypt"
)

const salt = "DiS!M4r#s$!&"

var (
	ErrInvalidData = errors.New("secret could not be verified: invalid data")
	ErrNotFound    = document.ErrNotFound
	ErrRevoked     = errors.New("secret is revoked")
)

// Store is a secrets store.
type Store struct {
	svc document.Service
}

// NewStore allocates storage for a new secrets store and initializes it.
func NewStore(svc document.Service) *Store {
	return &Store{svc: svc}
}

// SecretView is a view over a secret stored in the database.
type SecretView struct {
	ID        string
	Metadata  map[string]string
	Revoked   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// secret represents a secret.
type secret struct {
	ID        string
	Data      []byte
	Revoked   bool
	Metadata  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateSecret creates a secret with the given id and data.
func (s *Store) CreateSecret(
	ctx context.Context, id string, data []byte, metadata map[string]string,
) error {
	if id == "" {
		panic("invalid empty id")
	}

	// clone to add salt
	cloned := make([]byte, len(data)+len(salt))
	copy(cloned, data)
	copy(cloned[len(data):], salt)

	hashedData, err := bcrypt.GenerateFromPassword(data, bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("generate hash: %v", err)
	}

	now := time.Now()
	sec := secret{
		ID:        id,
		Data:      hashedData,
		Metadata:  metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.svc.Create(ctx, id, &sec); err != nil {
		return err
	}

	return nil
}

// VerifySecret returns ErrNotFound if a secret with the given id
// doesn't exist, or ErrInvalidData if the secret data doesn't match data.
// Also, it returns ErrRevoked if the secret has been revoked.
// It returns the metadata stored with the secret or nil of there's no metadata
// stored with the given secret.
func (s *Store) VerifySecret(
	ctx context.Context, id string, data []byte,
) (map[string]string, error) {
	if id == "" {
		panic("invalid empty id")
	}

	var sec secret
	if err := s.svc.Get(ctx, id, &sec); err != nil {
		if err == document.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("store get: %w", err)
	}

	if sec.Revoked {
		return nil, ErrRevoked
	}

	// clone to add salt
	cloned := make([]byte, len(data)+len(salt))
	copy(cloned, data)
	copy(cloned[len(data):], salt)

	err := bcrypt.CompareHashAndPassword(sec.Data, data)
	if err != nil {
		return nil, ErrInvalidData
	}
	return sec.Metadata, nil
}

// ListSecrets returns an iterator over the secrets stored in the underlying store.
func (s *Store) ListSecrets(
	ctx context.Context, filtersMap map[string]string,
) (iterator.Iterator[SecretView], error) {
	filters, err := makeDocumentSecretFilters(filtersMap)
	if err != nil {
		return nil, err
	}

	it, err := s.svc.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("store list: %w", err)
	}

	return iterator.Map(iterator.FromDocumentIterator[secret](it),
		func(s secret) SecretView {
			return SecretView{
				ID:        s.ID,
				Metadata:  s.Metadata,
				Revoked:   s.Revoked,
				CreatedAt: s.CreatedAt,
				UpdatedAt: s.UpdatedAt,
			}
		},
	), nil
}

// Revoke revokes the given secret or returns ErrNotFound if this secret doesn't exist.
// This method is idempotent.
func (s *Store) Revoke(ctx context.Context, id string) error {
	err := s.svc.Update(ctx, id, []document.Update{
		{FieldPath: []string{"Revoked"}, Value: true},
		{FieldPath: []string{document.DefaultUpdatedAtField}, Value: time.Now()},
	})
	if err == document.ErrNotFound {
		return ErrNotFound
	}
	return err
}

func makeDocumentSecretFilters(
	userFilters map[string]string,
) ([]document.Filter, error) {
	var ret []document.Filter
	for k, v := range userFilters {
		path := strings.Split(k, ".")
		switch path[0] {
		case "Revoked", "revoked":
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %q: %v", k, v)
			}
			ret = append(ret, document.Filter{
				Field: document.Field{
					FieldPath: path,
					Value:     b,
				},
				Op: document.OpEqual,
			})
		case "Metadata", "metadata":
			ret = append(ret, document.Filter{
				Field: document.Field{
					FieldPath: path,
					Value:     v,
				},
				Op: document.OpEqual,
			})
		default:
			return nil, fmt.Errorf("invalid field %q", k)
		}
	}
	return ret, nil
}
