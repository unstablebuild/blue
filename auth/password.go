package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
	"golang.org/x/crypto/bcrypt"
)

const salt = "DiS!M4r#s$!&"

var (
	ErrInvalidData = errors.New("password could not be verified: invalid data")
	ErrNotFound    = document.ErrNotFound
	ErrRevoked     = errors.New("password is revoked")
)

// PasswordStore stores password hashes for verification. It also allows
// passwords to be stored with metadata.
type PasswordStore struct {
	svc document.Service
}

// NewPasswordStore allocates storage for a new passwords store and initializes it.
func NewPasswordStore(svc document.Service) *PasswordStore {
	return &PasswordStore{svc: svc}
}

// PasswordView is a view over a password stored in the database.
type PasswordView struct {
	ID        string
	Metadata  map[string]string
	Revoked   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// password represents a password password.
type password struct {
	ID        string
	Data      []byte
	Revoked   bool
	Metadata  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreatePassword creates a password with the given id and data.
func (s *PasswordStore) CreatePassword(
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
	sec := password{
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

// VerifyPassword returns ErrNotFound if a password with the given id
// doesn't exist, or ErrInvalidData if the password data doesn't match data.
// Also, it returns ErrRevoked if the password has been revoked.
// It returns the metadata stored with the password or nil of there's no metadata
// stored with the given password.
func (s *PasswordStore) VerifyPassword(
	ctx context.Context, id string, data []byte,
) (map[string]string, error) {
	if id == "" {
		panic("invalid empty id")
	}

	var sec password
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

// ListPasswords returns an iterator over the passwords stored in the underlying store.
func (s *PasswordStore) ListPasswords(
	ctx context.Context, filtersMap map[string]string,
) (iterator.Iterator[PasswordView], error) {
	filters, err := makeDocumentPasswordFilters(filtersMap)
	if err != nil {
		return nil, err
	}

	it, err := s.svc.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("store list: %w", err)
	}

	return iterator.Map(iterator.FromDocumentIterator[password](it),
		func(s password) PasswordView {
			return PasswordView{
				ID:        s.ID,
				Metadata:  s.Metadata,
				Revoked:   s.Revoked,
				CreatedAt: s.CreatedAt,
				UpdatedAt: s.UpdatedAt,
			}
		},
	), nil
}

// Revoke revokes the given password or returns ErrNotFound if this password doesn't exist.
// This method is idempotent.
func (s *PasswordStore) Revoke(ctx context.Context, id string) error {
	err := s.svc.Update(ctx, id, []document.Update{
		{FieldPath: []string{"Revoked"}, Value: true},
		{FieldPath: []string{document.DefaultUpdatedAtField}, Value: time.Now()},
	})
	if err == document.ErrNotFound {
		return ErrNotFound
	}
	return err
}

func makeDocumentPasswordFilters(
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
