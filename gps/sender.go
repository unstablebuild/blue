package gps

import "io"

type Sender interface {
	Send(Coordinates) error
	io.Closer
}
