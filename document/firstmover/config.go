package firstmover

import (
	"time"

	"github.com/unstablebuild/blue/encoding"
	"github.com/unstablebuild/blue/encoding/bson"
)

// Config holds configuration for a firstmover document.Service
type Config struct {
	Marshaler encoding.Marshaler
	// TransientFailureRecoverTimeout is the timeout until a grpc
	// transient connection failure is considered unrecoverable..
	TransientFailureRecoverTimeout time.Duration
	// MethodRetryCadence is the method retry timeout until next attempt.
	MethodRetryCadence time.Duration
	// ConnectRetryCadence is the connect retry timeout until next attempt.
	ConnectRetryCadence time.Duration
	// TimeToCoup is the time for a follower to take the lead
	// if leader is unresponsive.
	TimeToCoup time.Duration
	// DialTimeout is net.Dial timeout
	DialTimeout time.Duration
	// CloseError can be optionally set to an error value that
	// the underlying document.Service passes when it's been
	// called Close and any other calls to its API will fail.
	// This error will be retried by followers until a new leader
	// is selected.
	CloseError error
}

// DefaultConfig returns a sane Config.
func DefaultConfig() Config {
	return Config{
		Marshaler:                      bson.Marshaler(),
		TransientFailureRecoverTimeout: 1 * time.Second,
		MethodRetryCadence:             20 * time.Millisecond,
		ConnectRetryCadence:            50 * time.Millisecond,
		TimeToCoup:                     1 * time.Second,
		DialTimeout:                    100 * time.Millisecond,
	}
}
