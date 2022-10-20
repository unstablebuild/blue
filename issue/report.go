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
	Build     debug.BuildInfo
	CreatedAt time.Time
	Closed    bool
	ClosedAt  time.Time
	Metadata  map[string]string
}

// IsInternalLabel can be used for Metadata keys to
// figure out if label is internal.
func IsInternalLabel(k string) bool {
	return strings.HasPrefix(k, "_")
}
