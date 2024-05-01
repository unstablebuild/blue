package release

//go:generate mockgen -destination=./manager_gomock.go -package release -self_package release -source manager.go

import (
	"context"
	"io"
	"time"

	"github.com/unstablebuild/blue/iterator"
)

// Package represents.. well, a package.
type Package struct {
	Name      string
	Latest    Version
	Notes     string
	Metadata  map[string]string
	CreatedAt time.Time `firestore:",serverTimestamp"`
}

// Version represents a package version.
type Version string

// Latest is a reserved Version that
// references the latest version uploaded
// for a package.
const Latest = "latest"

// Bundle represents a package release bundle.
type Bundle struct {
	Package   string
	Version   Version
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

// Manager abstracts the ability to manage packages and releases.
type Manager interface {
	// Create creates or updates a package.
	Create(context.Context, Package) error

	// DeletePackage deletes a package but package bundles are not deleted.
	// Note that this does not prevent the next call to Upload to fail.
	// In order to delete all traces of a package, all bundles must be deleted
	// first via Delete.
	DeletePackage(context.Context, string) error

	// Get fetches a Package manifest.
	GetPackage(context.Context, string) (Package, error)

	// ListPackages lists all packages.
	ListPackages(context.Context, map[string]string) (iterator.Iterator[Package], error)

	// Upload uploads a package bundle and reports progress via ProgressReader.
	// It updates the Latest field of the Package.
	// If package has not been created via Create then it creates a new one.
	Upload(context.Context, Bundle, ProgressReader) error

	// Get downloads a Package bundle by package name and version and reports
	// progress via ProgressWriter.
	Get(context.Context, string, Version, ProgressWriter) (Bundle, error)

	// Delete deletes a package bundle by package name and version.
	Delete(context.Context, string, Version) error

	// List lists all bundles of a package.
	List(context.Context, string, map[string]string) (iterator.Iterator[Bundle], error)
}
