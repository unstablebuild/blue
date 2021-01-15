package release

//go:generate mockgen -destination=./manager_gomock.go -package release -self_package release -source manager.go

import (
	"context"
	"io"
	"time"
)

type Manifest struct {
	ID        string
	Notes     string
	Metadata  map[string]string
	CreatedAt time.Time `firestore:",serverTimestamp"`
}

type Manager interface {
	Create(context.Context, Manifest, io.Reader) error
	Get(context.Context, string, io.Writer) (Manifest, error)
	Delete(context.Context, string) error
	List(context.Context, map[string]string) ([]Manifest, error)
}
