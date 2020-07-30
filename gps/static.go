package gps

import (
	"context"
	"time"
)

type staticPositioner struct {
	pos Coordinates
}

// NewStaticPositioner returns a Positioner that always yields the same
// GPS position.
func NewStaticPositioner(pos Coordinates) Positioner {
	return staticPositioner{pos: pos}
}

func (p staticPositioner) Position(ctx context.Context) (Coordinates, error) {
	pos := p.pos
	pos.UnixTime = time.Now().Unix()
	return pos, nil
}

func (p staticPositioner) Close() error {
	return nil
}
