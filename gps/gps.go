package gps

import (
	"context"
	"io"
)

// DefaultPort is the default port used by GPS servers and clients.
const DefaultPort = 4677

// Coordinates represent a set GPS coordinates represented by a latitude and longitude.
type Coordinates struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
}

// Positioner wraps the basic function Position.
type Positioner interface {
	Position(context.Context) (Coordinates, error)
	io.Closer
}
