package gps

import (
	"context"
	"io"
)

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
