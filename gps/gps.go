package gps

import (
	"context"
	"io"
	"time"
)

// Coordinates represent a set GPS coordinates represented by a latitude and longitude.
type Coordinates struct {
	DeviceID  string
	Latitude  float32
	Longitude float32
	Altitude  float32
	Time      time.Time
}

// Positioner wraps the basic function Position.
type Positioner interface {
	// Positions locates the gps.Coordinates of the current process.
	Position(context.Context) (Coordinates, error)
	io.Closer
}

// Sender wraps the basic function Send.
type Sender interface {
	// Send sends the gps.Coordinates to a target. Implementers MUST perform
	// connection setup during this method call, and not in their constructors.
	Send(context.Context, Coordinates) error
	io.Closer
}

// Receiver manages the lifecycle of a connection-based GPS receiver.
type Receiver interface {
	Receive(context.Context, Coordinates) error
}
