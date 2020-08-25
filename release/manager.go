package release

import (
	"context"
	"io"
	"time"
)

type Manifest struct {
	ID        string
	Author    string
	Notes     string
	CreatedAt time.Time `firestore:",serverTimestamp"`
}

type Manager interface {
	Create(context.Context, Manifest, io.Reader) error
	Get(context.Context, string, io.Writer) (Manifest, error)
	Delete(context.Context, string) error
	List(context.Context) ([]Manifest, error)
}
