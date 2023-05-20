package issue

import (
	"runtime/debug"
	"strings"
	"time"
)

// Report contains informatin about a suspected or confirmed bug.
type Report struct {
	Package   string
	Version   string
	Author    string
	Subject   string
	Notes     string
	Build     debug.BuildInfo `yaml:"build,omitempty"`
	CreatedAt time.Time       `yaml:"created_at,omitempty"`
	UpdatedAt time.Time       `yaml:"updated_at,omitempty"`
	Closed    bool            `yaml:"closed,omitempty"`
	ClosedAt  time.Time       `yaml:"closed_at,omitempty"`
	Metadata  map[string]string
}

// IsInternalLabel can be used for Metadata keys to
// figure out if label is internal.
func IsInternalLabel(k string) bool {
	return strings.HasPrefix(k, "_")
}
