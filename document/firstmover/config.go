package firstmover

import (
	"time"
)

// Config holds configuration for a firstmover document.Service
type Config struct {
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
}

// DefaultConfig returns a sane Config.
func DefaultConfig() Config {
	return Config{
		TransientFailureRecoverTimeout: 1 * time.Second,
		MethodRetryCadence:             20 * time.Millisecond,
		ConnectRetryCadence:            50 * time.Millisecond,
		TimeToCoup:                     1 * time.Second,
		DialTimeout:                    100 * time.Millisecond,
	}
}
