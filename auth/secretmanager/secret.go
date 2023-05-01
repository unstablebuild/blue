package secretmanager

import "time"

// State can be one of enabled, disabled or destroyed.
type State string

const (
	StateEnabled   State = "enabled"
	StateDisabled  State = "disabled"
	StateDestroyed State = "destroyed"
	StateUnknown   State = "unknown"
)

// SecretVersion represents a securely stored secret version.
type SecretVersion struct {
	ID        string
	Version   string
	State     State
	CreatedAt time.Time `json:"created_at,omitempty"`
	Payload   []byte
}

// Secret represents a logical secret whose values can be accessed.
type Secret struct {
	ID          string
	CreatedAt   time.Time
	Annotations map[string]string
}
