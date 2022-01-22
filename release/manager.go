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

// ProgressReader wraps io.Reader to add a Progress hook so
// implementations are able to provide a mechanism for clients
// to know about the current Create progress.
type ProgressReader interface {
	io.Reader
	Progress(progress, total int64, units string)
}

// ProgressWriter wraps io.Writer to add a Progress hook so
// implementations are able to provide a mechanism for clients
// to know about the current Get progress.
type ProgressWriter interface {
	io.Writer
	Progress(progress, total int64, units string)
}

type Manager interface {
	Create(context.Context, Manifest, ProgressReader) error
	Get(context.Context, string, ProgressWriter) (Manifest, error)
	Delete(context.Context, string) error
	List(context.Context, map[string]string) ([]Manifest, error)
}
